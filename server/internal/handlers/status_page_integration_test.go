package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	statuspagesvc "github.com/serversupervisor/server/internal/services/statuspage"
	uptimesvc "github.com/serversupervisor/server/internal/services/uptime"
	"github.com/serversupervisor/server/internal/testutil"
)

func newStatusPageRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	uh := handlers.NewUptimeHandler(uptimesvc.NewService(db))
	sh := handlers.NewStatusPageHandler(statuspagesvc.NewService(db), t.TempDir()+"/missing-index.html")
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("username", "admin"); c.Next() })
	r.POST("/uptime/probes", uh.Create)
	r.PUT("/uptime/probes/:id", uh.Update)
	r.GET("/uptime/probes/:id", uh.Get)
	r.GET("/uptime/sla", uh.SLA)
	r.GET("/status-page", sh.GetConfig)
	r.PUT("/status-page", sh.UpdateConfig)
	r.GET("/api/public/status", sh.Public)
	r.GET("/status", sh.Page)
	return r, db
}

func createProbe(t *testing.T, r http.Handler, body map[string]any) string {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/uptime/probes", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create probe: %d %s", w.Code, w.Body.String())
	}
	var out idOnly
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.ID
}

func record(t *testing.T, db *database.DB, probeID string, at time.Time, ok bool, errText string) {
	t.Helper()
	if err := db.RecordUptimeProbeResult(context.Background(), models.UptimeProbeResult{
		ProbeID: probeID, CheckedAt: at, Success: ok, LatencyMs: 12, Error: errText,
	}); err != nil {
		t.Fatalf("record result: %v", err)
	}
}

func TestUptimeProbe_SLOTargetDefaultAndKeep(t *testing.T) {
	r, _ := newStatusPageRouter(t)
	id := createProbe(t, r, map[string]any{"type": "tcp", "name": "db", "target": "db.internal:5432"})

	var p models.UptimeProbe
	_ = json.Unmarshal(doJSON(t, r, http.MethodGet, "/uptime/probes/"+id, nil).Body.Bytes(), &p)
	if p.SLOTarget != models.DefaultSLOTarget {
		t.Fatalf("default objective = %v, want %v", p.SLOTarget, models.DefaultSLOTarget)
	}

	w := doJSON(t, r, http.MethodPut, "/uptime/probes/"+id, map[string]any{"type": "tcp", "name": "db", "target": "db.internal:5432", "slo_target": 99.95})
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if w.Code != http.StatusOK || p.SLOTarget != 99.95 {
		t.Fatalf("set objective: %d %v", w.Code, p.SLOTarget)
	}
	// An update without slo_target keeps the stored objective.
	w = doJSON(t, r, http.MethodPut, "/uptime/probes/"+id, map[string]any{"type": "tcp", "name": "db2", "target": "db.internal:5432"})
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p.SLOTarget != 99.95 || p.Name != "db2" {
		t.Fatalf("objective must be kept on update, got %v", p.SLOTarget)
	}
	if w := doJSON(t, r, http.MethodPut, "/uptime/probes/"+id, map[string]any{"type": "tcp", "name": "db", "target": "x:1", "slo_target": 101}); w.Code != http.StatusBadRequest {
		t.Fatalf("objective above 100 must be a 400, got %d", w.Code)
	}
}

func TestUptimeSLAEndpoint_ReportsCoverage(t *testing.T) {
	r, db := newStatusPageRouter(t)
	id := createProbe(t, r, map[string]any{"type": "tcp", "name": "api", "target": "api.internal:443"})

	today := time.Now().UTC().Truncate(24 * time.Hour)
	threeDaysAgo := today.AddDate(0, 0, -3).Add(6 * time.Hour)
	record(t, db, id, threeDaysAgo, true, "")
	record(t, db, id, threeDaysAgo.Add(time.Minute), false, "refused")
	record(t, db, id, today.Add(time.Minute), true, "")
	if _, err := db.RollupUptimeDaily(context.Background(), 29); err != nil {
		t.Fatalf("rollup: %v", err)
	}

	w := doJSON(t, r, http.MethodGet, "/uptime/sla", nil)
	var report models.SLAReport
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || w.Code != http.StatusOK {
		t.Fatalf("sla: %d %s", w.Code, w.Body.String())
	}
	if len(report.Probes) != 1 {
		t.Fatalf("expected one probe, got %+v", report.Probes)
	}
	week := report.Probes[0].Windows[0]
	if week.Days != 7 || week.CoveredDays != 2 || week.TotalChecks != 3 || week.UpChecks != 2 || week.Status != models.SLAStatusBreached {
		t.Fatalf("unexpected 7-day window %+v", week)
	}
}

func TestStatusPage_PublicEndpoint(t *testing.T) {
	r, db := newStatusPageRouter(t)
	pub := createProbe(t, r, map[string]any{"type": "http", "name": "prod-web-01", "target": "http://192.168.10.5:8080/healthz"})
	secret := createProbe(t, r, map[string]any{"type": "tcp", "name": "backup-nas", "target": "nas.lan:22"})
	now := time.Now().UTC()
	record(t, db, pub, now.Add(-time.Minute), false, "dial tcp 192.168.10.5:8080: connection refused")
	record(t, db, secret, now.Add(-time.Minute), false, "ssh banner timeout")

	// Disabled by default: both the JSON endpoint and the SPA entry are 404.
	if w := doJSON(t, r, http.MethodGet, "/api/public/status", nil); w.Code != http.StatusNotFound {
		t.Fatalf("disabled page must be 404, got %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodGet, "/status", nil); w.Code != http.StatusNotFound {
		t.Fatalf("disabled SPA entry must be 404, got %d", w.Code)
	}

	bad := doJSON(t, r, http.MethodPut, "/status-page", map[string]any{
		"enabled": true, "services": []map[string]any{{"probe_id": "00000000-0000-0000-0000-000000000000", "public_name": "Ghost"}},
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("unknown probe must be a 400, got %d", bad.Code)
	}

	w := doJSON(t, r, http.MethodPut, "/status-page", map[string]any{
		"enabled": true, "title": "Acme", "services": []map[string]any{{"probe_id": pub, "public_name": "Website"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update config: %d %s", w.Code, w.Body.String())
	}
	var cfg models.StatusPageConfig
	_ = json.Unmarshal(doJSON(t, r, http.MethodGet, "/status-page", nil).Body.Bytes(), &cfg)
	if !cfg.Enabled || len(cfg.Services) != 1 || cfg.Services[0].PublicName != "Website" {
		t.Fatalf("config not stored: %+v", cfg)
	}

	pw := doJSON(t, r, http.MethodGet, "/api/public/status", nil)
	if pw.Code != http.StatusOK || pw.Header().Get("Cache-Control") != "public, max-age=30" {
		t.Fatalf("public page: %d cache=%q", pw.Code, pw.Header().Get("Cache-Control"))
	}
	body := pw.Body.String()
	for _, leak := range []string{"192.168", "prod-web-01", "connection refused", "backup-nas", "nas.lan", "ssh banner", pub, secret, "healthz"} {
		if strings.Contains(body, leak) {
			t.Errorf("public payload leaks %q", leak)
		}
	}
	var page models.PublicStatusPage
	_ = json.Unmarshal(pw.Body.Bytes(), &page)
	if len(page.Services) != 1 || page.Services[0].Name != "Website" || page.Status != models.PublicStatusMajorOutage {
		t.Fatalf("unexpected public page %+v", page)
	}
	if len(page.Incidents) != 1 || page.Incidents[0].Service != "Website" || page.Incidents[0].ResolvedAt != nil {
		t.Fatalf("expected one ongoing generic incident, got %+v", page.Incidents)
	}
	if w := doJSON(t, r, http.MethodGet, "/status", nil); w.Code != http.StatusOK {
		t.Fatalf("enabled SPA entry must be 200, got %d", w.Code)
	}

	// Deleting a published probe unpublishes it (FK cascade).
	if _, err := db.Exec(context.Background(), `DELETE FROM uptime_probes WHERE id = $1`, pub); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(doJSON(t, r, http.MethodGet, "/status-page", nil).Body.Bytes(), &cfg)
	if len(cfg.Services) != 0 {
		t.Fatalf("deleted probe must leave the page, got %+v", cfg.Services)
	}
}
