package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	discoverysvc "github.com/serversupervisor/server/internal/services/discovery"
)

type discoveryRepoStub struct {
	commands map[string]*models.RemoteCommand
}

func (r *discoveryRepoStub) GetHost(_ context.Context, id string) (*models.Host, error) {
	if id != "h1" {
		return nil, errors.New("no rows")
	}
	return &models.Host{ID: "h1", Collectors: map[string]bool{"network_discovery": true, "network_scan": true}}, nil
}
func (r *discoveryRepoStub) GetAllHosts(context.Context) ([]models.Host, error) { return nil, nil }
func (r *discoveryRepoStub) GetRemoteCommandByID(_ context.Context, id string) (*models.RemoteCommand, error) {
	if c, ok := r.commands[id]; ok {
		return c, nil
	}
	return nil, errors.New("no rows")
}
func (r *discoveryRepoStub) GetRemoteCommandsByHostAndModule(context.Context, string, string, int) ([]models.RemoteCommand, error) {
	return nil, nil
}
func (r *discoveryRepoStub) CancelPendingRemoteCommand(context.Context, string, string) (bool, error) {
	return false, nil
}
func (r *discoveryRepoStub) CreateAuditLog(context.Context, string, string, string, string, string, string) (int64, error) {
	return 1, nil
}

type discoveryDispatchStub struct{ payloads []string }

func (d *discoveryDispatchStub) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	d.payloads = append(d.payloads, req.Payload)
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "job-1"}}, nil
}

func newAgentDiscoveryRouter(role string) (*gin.Engine, *discoveryDispatchStub) {
	repo := &discoveryRepoStub{commands: map[string]*models.RemoteCommand{
		"run": {ID: "run", Module: "discovery", Action: "port_scan", Status: "running"},
	}}
	disp := &discoveryDispatchStub{}
	h := handlers.NewDiscoveryHandler(nil, discoverysvc.NewAgentService(repo, disp, &config.Config{}, nil))
	r := gin.New()
	r.Use(withRole(role))
	r.GET("/discovery/limits", h.Limits)
	r.POST("/discovery/arp", h.StartARP)
	r.POST("/discovery/port-scan", h.StartPortScan)
	r.POST("/discovery/snmp", h.StartSNMP)
	r.GET("/discovery/jobs/:id", h.GetJob)
	r.POST("/discovery/jobs/:id/cancel", h.CancelJob)
	r.POST("/hosts/discover", h.Scan)
	return r, disp
}

func doDiscovery(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestAgentDiscoveryEndpointsAreAdminOnly(t *testing.T) {
	r, _ := newAgentDiscoveryRouter(models.RoleOperator)
	for _, ep := range [][2]string{
		{"GET", "/discovery/limits"}, {"POST", "/discovery/arp"}, {"POST", "/discovery/port-scan"},
		{"POST", "/discovery/snmp"}, {"GET", "/discovery/jobs/run"}, {"POST", "/discovery/jobs/run/cancel"},
		{"POST", "/hosts/discover"},
	} {
		if w := doDiscovery(r, ep[0], ep[1], `{}`); w.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d", ep[0], ep[1], w.Code)
		}
	}
}

func TestAgentDiscoveryEndpoints(t *testing.T) {
	r, disp := newAgentDiscoveryRouter(models.RoleAdmin)
	const community = "test-placeholder"

	cases := []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/discovery/limits", "", http.StatusOK},
		{"POST", "/discovery/arp", `{"host_id":"h1"}`, http.StatusAccepted},
		{"POST", "/discovery/arp", `{}`, http.StatusBadRequest},
		{"POST", "/discovery/arp", `{"host_id":"nope"}`, http.StatusNotFound},
		{"POST", "/discovery/port-scan", `{"host_id":"h1","cidr":"192.168.1.0/24","ports":"22"}`, http.StatusAccepted},
		{"POST", "/discovery/port-scan", `{"host_id":"h1","cidr":"8.8.8.0/24"}`, http.StatusBadRequest},
		{"POST", "/discovery/port-scan", `nope`, http.StatusBadRequest},
		{"POST", "/discovery/snmp", `{"host_id":"h1","target":"192.168.1.9","version":"2c","community":"` + community + `"}`, http.StatusAccepted},
		{"POST", "/discovery/snmp", `nope`, http.StatusBadRequest},
		{"POST", "/discovery/snmp", `{"host_id":"h1","target":"192.168.1.9","version":"9"}`, http.StatusBadRequest},
		{"GET", "/discovery/jobs/run", "", http.StatusOK},
		{"GET", "/discovery/jobs/missing", "", http.StatusNotFound},
		{"POST", "/discovery/jobs/run/cancel", "", http.StatusOK},
		{"POST", "/discovery/jobs/missing/cancel", "", http.StatusNotFound},
	}
	for _, c := range cases {
		w := doDiscovery(r, c.method, c.path, c.body)
		if w.Code != c.code {
			t.Errorf("%s %s %s: %d %s", c.method, c.path, c.body, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), community) {
			t.Errorf("%s: response echoes the community", c.path)
		}
	}
	for _, p := range disp.payloads {
		if strings.Contains(p, community) {
			t.Fatalf("community persisted in payload: %s", p)
		}
	}
}
