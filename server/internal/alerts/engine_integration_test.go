package alerts_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// stubPusher satisfies alerts.NotificationPusher without doing anything.
type stubPusher struct{ count int }

func (s *stubPusher) Broadcast(_ interface{}) { s.count++ }

func insertCPUMetric(t *testing.T, db *database.DB, hostID string, cpu float64, ts time.Time) {
	t.Helper()
	if _, err := db.InsertMetrics(context.Background(), &models.SystemMetrics{
		HostID:          hostID,
		Timestamp:       ts,
		CPUUsagePercent: cpu,
		Hostname:        "alert-host",
	}); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
}

// TestEvaluateAlerts_CreatesAndResolvesIncident exercises the full alert
// evaluation cycle against a real database: a CPU rule fires an incident when
// the latest metric breaches the threshold, then resolves once it recovers.
func TestEvaluateAlerts_CreatesAndResolvesIncident(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-1"
	if err := db.RegisterHost(ctx, &models.Host{
		ID:       hostID,
		Name:     "alert-host",
		Hostname: "alert-host",
		Status:   "online",
		LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	// Breaching metric: CPU well above the warn threshold.
	insertCPUMetric(t, db, hostID, 95, time.Now())

	warn := 50.0
	rule := &models.AlertRule{
		SourceType:    "agent",
		HostID:        &hostID,
		Metric:        "cpu",
		Operator:      ">",
		ThresholdWarn: &warn,
		Enabled:       true,
		Actions:       models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := &config.Config{}
	disp := dispatch.New(db)
	pusher := &stubPusher{}

	// First evaluation: an incident should be opened at warn severity.
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher)

	inc, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected an open incident after a threshold breach, got error: %v", err)
	}
	if inc.Severity != "warn" {
		t.Errorf("incident severity = %q, want warn", inc.Severity)
	}

	// Recovery: a newer metric below the threshold becomes the latest sample.
	insertCPUMetric(t, db, hostID, 10, time.Now().Add(2*time.Second))

	// Second evaluation: the open incident should be resolved.
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher)

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err == nil {
		t.Error("expected the incident to be resolved after CPU recovered, but one is still open")
	}
}

// TestEvaluateAlerts_NoIncidentBelowThreshold ensures a healthy metric never
// opens an incident.
func TestEvaluateAlerts_NoIncidentBelowThreshold(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-2"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "alert-host", Hostname: "alert-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	insertCPUMetric(t, db, hostID, 5, time.Now())

	warn := 50.0
	rule := &models.AlertRule{
		SourceType: "agent", HostID: &hostID, Metric: "cpu", Operator: ">",
		ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	alerts.EvaluateAlerts(ctx, db, &config.Config{}, dispatch.New(db), &stubPusher{})

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err == nil {
		t.Error("did not expect an incident for a metric below the threshold")
	}
}
