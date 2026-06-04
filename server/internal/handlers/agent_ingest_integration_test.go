package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// newAgentReportRouter wires the report endpoint with a stub auth middleware
// that injects hostID into the context (the real APIKeyMiddleware is exercised
// elsewhere). Passing an empty hostID simulates an unauthenticated request.
func newAgentReportRouter(t *testing.T, hostID string) (*gin.Engine, *database.DB) {
	t.Helper()
	db := testutil.NewPostgresDB(t)
	h := handlers.NewAgentHandler(db, &config.Config{APIKeyHeader: "X-API-Key"}, nil, nil)

	r := gin.New()
	r.POST("/report", func(c *gin.Context) {
		if hostID != "" {
			c.Set("host_id", hostID)
		}
		c.Next()
	}, h.ReceiveReport)
	return r, db
}

// seedHost inserts a host row so foreign-keyed inserts (metrics, containers…)
// succeed. Status starts offline so the online transition can be asserted.
func seedHost(t *testing.T, db *database.DB, id string) {
	t.Helper()
	if err := db.RegisterHost(context.Background(), &models.Host{
		ID:       id,
		Name:     "test-host",
		Hostname: "test-host",
		Status:   "offline",
		APIKey:   "irrelevant-for-ingest-test",
	}); err != nil {
		t.Fatalf("seed host: %v", err)
	}
}

func postReport(t *testing.T, r http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s) // raw payload (malformed-JSON case)
	} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode report: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/report", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type reportResponse struct {
	Status      string                  `json:"status"`
	SkipMetrics bool                    `json:"skip_metrics"`
	Commands    []models.PendingCommand `json:"commands"`
}

func TestReceiveReport_PersistsMetricsDockerDisk(t *testing.T) {
	const hostID = "host-ingest-1"
	r, db := newAgentReportRouter(t, hostID)
	seedHost(t, db, hostID)

	report := models.AgentReport{
		AgentVersion: "9.9.9",
		Capabilities: &models.AgentCapabilities{Docker: true, SMART: true},
		Metrics: &models.SystemMetrics{
			CPUUsagePercent: 42.5,
			CPUModel:        "Test CPU",
			MemoryTotal:     1000,
			MemoryUsed:      500,
			MemoryPercent:   50,
			Uptime:          12345,
			Hostname:        "test-host",
			OS:              "Debian 12",
		},
		Docker: &models.DockerReport{
			Containers: []models.DockerContainer{{
				ID:          "abc123",
				ContainerID: "abc123",
				Name:        "nginx",
				Image:       "nginx",
				ImageTag:    "1.27",
				State:       "running",
				Status:      "Up 2 hours",
			}},
		},
		DiskHealth: []models.DiskHealth{{
			Device:       "/dev/sda",
			Model:        "TestDisk",
			SmartStatus:  "PASSED",
			Temperature:  35,
			PowerOnHours: 100,
			PowerCycles:  10,
		}},
		Timestamp: time.Now(),
	}

	w := postReport(t, r, report)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp reportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("response status = %q, want ok", resp.Status)
	}
	if resp.SkipMetrics {
		t.Error("skip_metrics should be false for a host without a Proxmox metrics source")
	}
	if len(resp.Commands) != 0 {
		t.Errorf("commands = %d, want 0", len(resp.Commands))
	}

	ctx := context.Background()

	metrics, err := db.GetMetricsHistory(ctx, hostID, 24)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("metrics rows = %d, want 1", len(metrics))
	}
	if metrics[0].CPUUsagePercent != 42.5 {
		t.Errorf("stored CPU = %v, want 42.5", metrics[0].CPUUsagePercent)
	}

	containers, err := db.GetDockerContainers(ctx, hostID)
	if err != nil {
		t.Fatalf("get containers: %v", err)
	}
	if len(containers) != 1 || containers[0].Name != "nginx" {
		t.Fatalf("containers = %+v, want one nginx", containers)
	}

	disks, err := db.GetLatestDiskHealth(ctx, hostID)
	if err != nil {
		t.Fatalf("get disk health: %v", err)
	}
	if len(disks) != 1 || disks[0].Device != "/dev/sda" || disks[0].PowerCycles != 10 {
		t.Fatalf("disk health = %+v, want /dev/sda with power_cycles=10", disks)
	}

	// The host should have transitioned offline -> online during ingest.
	if status := db.GetHostStatus(ctx, hostID); status != "online" {
		t.Errorf("host status = %q, want online", status)
	}
}

func TestReceiveReport_ReturnsPendingCommands(t *testing.T) {
	const hostID = "host-ingest-2"
	r, db := newAgentReportRouter(t, hostID)
	seedHost(t, db, hostID)

	cmd, err := db.CreateRemoteCommand(context.Background(), hostID, "docker", "restart", "nginx", "{}", "tester", nil)
	if err != nil {
		t.Fatalf("seed command: %v", err)
	}

	w := postReport(t, r, models.AgentReport{
		AgentVersion: "1.0.0",
		Metrics:      &models.SystemMetrics{CPUUsagePercent: 1, Hostname: "test-host"},
		Timestamp:    time.Now(),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	var resp reportResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(resp.Commands))
	}
	got := resp.Commands[0]
	if got.ID != cmd.ID || got.Module != "docker" || got.Action != "restart" || got.Target != "nginx" {
		t.Errorf("unexpected command returned: %+v", got)
	}
}

func TestReceiveReport_Unauthorized(t *testing.T) {
	r, _ := newAgentReportRouter(t, "") // empty hostID -> middleware sets nothing
	w := postReport(t, r, models.AgentReport{AgentVersion: "1.0.0", Timestamp: time.Now()})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body = %s", w.Code, w.Body.String())
	}
}

func TestReceiveReport_BadJSON(t *testing.T) {
	r, db := newAgentReportRouter(t, "host-ingest-3")
	seedHost(t, db, "host-ingest-3")
	w := postReport(t, r, "{not valid json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}
