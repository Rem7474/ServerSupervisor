package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// taskLogRepo implements only the two Repository methods nodeClient needs;
// the embedded interface is nil, so anything else would panic loudly rather
// than silently returning a zero value.
type taskLogRepo struct {
	Repository
	apiURL string
}

func (r taskLogRepo) GetProxmoxNode(context.Context, string) (*models.ProxmoxNode, error) {
	return &models.ProxmoxNode{ID: "node-1", ConnectionID: "conn-1", NodeName: "pve1"}, nil
}

func (r taskLogRepo) GetProxmoxConnectionByID(context.Context, string) (*models.ProxmoxConnection, error) {
	return &models.ProxmoxConnection{ID: "conn-1", APIURL: r.apiURL, TokenID: "u!t"}, nil
}

func (r taskLogRepo) GetEnabledProxmoxConnections(context.Context) ([]database.ProxmoxConnectionFull, error) {
	return []database.ProxmoxConnectionFull{{
		ProxmoxConnection: models.ProxmoxConnection{ID: "conn-1", APIURL: r.apiURL, TokenID: "u!t"},
		TokenSecret:       "secret",
	}}, nil
}

// pveTaskLogServer serves a paginated log of `total` lines whose last line is
// the closing task marker, plus a status endpoint.
func pveTaskLogServer(t *testing.T, total int, statusBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes/pve1/tasks/UPID:pve1:X/log", func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		lines := make([]string, 0, limit)
		for i := start; i < start+limit && i < total; i++ {
			text := fmt.Sprintf("line %d", i+1)
			if i == total-1 {
				text = "TASK ERROR: job errors"
			}
			lines = append(lines, fmt.Sprintf(`{"n":%d,"t":%q}`, i+1, text))
		}
		_, _ = fmt.Fprintf(w, `{"total":%d,"data":[%s]}`, total, strings.Join(lines, ","))
	})
	mux.HandleFunc("/nodes/pve1/tasks/UPID:pve1:X/status", func(w http.ResponseWriter, r *http.Request) {
		if statusBody == "" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		_, _ = w.Write([]byte(statusBody))
	})
	return httptest.NewServer(mux)
}

func TestServiceTaskLogReportsPVEsOwnLifecycle(t *testing.T) {
	pve := pveTaskLogServer(t, 1200, `{"data":{"status":"stopped","exitstatus":"job errors"}}`)
	defer pve.Close()

	svc := &Service{repo: taskLogRepo{apiURL: pve.URL}}
	got, err := svc.TaskLog(context.Background(), "node-1", "UPID:pve1:X")
	if err != nil {
		t.Fatalf("TaskLog: %v", err)
	}

	// The whole point of the fix: a task that ended is reported as ended, and
	// the log window shows the end rather than the first 50 lines.
	if !got.Finished {
		t.Error("Finished = false for a stopped task")
	}
	if got.ExitStatus != "job errors" {
		t.Errorf("ExitStatus = %q, want %q", got.ExitStatus, "job errors")
	}
	if got.Total != 1200 || !got.Truncated {
		t.Errorf("Total = %d, Truncated = %v, want 1200 / true", got.Total, got.Truncated)
	}
	if len(got.Lines) == 0 || got.Lines[len(got.Lines)-1].T != "TASK ERROR: job errors" {
		t.Error("the closing task marker is missing from the returned window")
	}
}

func TestServiceTaskLogShortLogIsNotFlaggedTruncated(t *testing.T) {
	pve := pveTaskLogServer(t, 3, `{"data":{"status":"stopped","exitstatus":"OK"}}`)
	defer pve.Close()

	svc := &Service{repo: taskLogRepo{apiURL: pve.URL}}
	got, err := svc.TaskLog(context.Background(), "node-1", "UPID:pve1:X")
	if err != nil {
		t.Fatalf("TaskLog: %v", err)
	}
	if got.Truncated || len(got.Lines) != 3 {
		t.Errorf("Truncated = %v with %d lines, want false / 3", got.Truncated, len(got.Lines))
	}
}

func TestServiceTaskLogStillReturnsLinesWhenStatusIsUnavailable(t *testing.T) {
	// An unreadable status must not cost the user the console, and must not
	// claim a running task has finished.
	pve := pveTaskLogServer(t, 10, "")
	defer pve.Close()

	svc := &Service{repo: taskLogRepo{apiURL: pve.URL}}
	got, err := svc.TaskLog(context.Background(), "node-1", "UPID:pve1:X")
	if err != nil {
		t.Fatalf("TaskLog: %v", err)
	}
	if len(got.Lines) != 10 {
		t.Errorf("got %d lines, want the log despite the status failure", len(got.Lines))
	}
	if got.Finished {
		t.Error("Finished = true although the lifecycle could not be read")
	}
}
