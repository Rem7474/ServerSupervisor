package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	systemsvc "github.com/serversupervisor/server/internal/services/system"
)

type recordingDispatcher struct {
	got dispatch.Request
	err error
}

func (r *recordingDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	r.got = req
	if r.err != nil {
		return nil, r.err
	}
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-42"}}, nil
}

// callSystem runs one SystemHandler endpoint as a caller with the given role
// and returns the recorded response.
func callSystem(t *testing.T, role string, access fakeHostAccess, d *recordingDispatcher, endpoint func(*SystemHandler) gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewSystemHandler(access, systemsvc.NewService(d))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", role)
	c.Set("username", "alice")
	endpoint(h)(c)
	return w
}

var systemEndpoints = map[string]struct {
	fn   func(*SystemHandler) gin.HandlerFunc
	body string
}{
	"journal":   {func(h *SystemHandler) gin.HandlerFunc { return h.SendJournalCommand }, `{"host_id":"h1","service_name":"nginx.service"}`},
	"processes": {func(h *SystemHandler) gin.HandlerFunc { return h.SendProcessesCommand }, `{"host_id":"h1"}`},
	"systemd":   {func(h *SystemHandler) gin.HandlerFunc { return h.SendSystemdCommand }, `{"host_id":"h1","service_name":"nginx.service","action":"restart"}`},
}

func TestSystemHandler_QueuesCommandForOperator(t *testing.T) {
	for name, ep := range systemEndpoints {
		t.Run(name, func(t *testing.T) {
			d := &recordingDispatcher{}
			w := callSystem(t, models.RoleOperator, fakeHostAccess{}, d, ep.fn, ep.body)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"command_id":"cmd-42"`) || !strings.Contains(w.Body.String(), `"status":"pending"`) {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			if d.got.HostID != "h1" || d.got.Audit == nil || d.got.Audit.Username != "alice" {
				t.Errorf("unexpected dispatched request: %+v", d.got)
			}
		})
	}
}

func TestSystemHandler_RejectsViewer(t *testing.T) {
	for name, ep := range systemEndpoints {
		t.Run(name, func(t *testing.T) {
			d := &recordingDispatcher{}
			w := callSystem(t, models.RoleViewer, fakeHostAccess{}, d, ep.fn, ep.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d", w.Code)
			}
			if d.got.Module != "" {
				t.Error("nothing must be dispatched for a viewer")
			}
		})
	}
}

func TestSystemHandler_RejectsMalformedBody(t *testing.T) {
	for name, ep := range systemEndpoints {
		t.Run(name, func(t *testing.T) {
			w := callSystem(t, models.RoleAdmin, fakeHostAccess{}, &recordingDispatcher{}, ep.fn, `{}`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", w.Code)
			}
		})
	}
}

func TestSystemHandler_EnforcesHostLevelAccess(t *testing.T) {
	// A restricted operator holding only viewer rights on the host is denied.
	restrictedViewer := fakeHostAccess{restricted: true, level: "viewer"}
	for name, ep := range systemEndpoints {
		t.Run(name, func(t *testing.T) {
			d := &recordingDispatcher{}
			w := callSystem(t, models.RoleOperator, restrictedViewer, d, ep.fn, ep.body)
			if w.Code != http.StatusForbidden {
				t.Fatalf("want 403, got %d", w.Code)
			}
			if d.got.Module != "" {
				t.Error("nothing must be dispatched without host access")
			}
		})
	}
}

func TestSystemHandler_SurfacesServiceErrors(t *testing.T) {
	for name, ep := range systemEndpoints {
		t.Run(name, func(t *testing.T) {
			d := &recordingDispatcher{err: errors.New("queue down")}
			w := callSystem(t, models.RoleAdmin, fakeHostAccess{}, d, ep.fn, ep.body)
			if w.Code < 500 {
				t.Fatalf("want a 5xx when dispatch fails, got %d", w.Code)
			}
		})
	}
}

func TestSystemHandler_ValidationErrorFromServiceIs400(t *testing.T) {
	d := &recordingDispatcher{}
	ep := systemEndpoints["journal"]
	w := callSystem(t, models.RoleAdmin, fakeHostAccess{}, d, ep.fn, `{"host_id":"h1","service_name":"nginx; rm -rf /"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
