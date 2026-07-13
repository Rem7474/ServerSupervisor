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
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)

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
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err == nil {
		t.Error("expected the incident to be resolved after CPU recovered, but one is still open")
	}
}

// TestEvaluateAlerts_CooldownSuppressesRenotification checks that
// AlertActions.Cooldown suppresses the loud notification channels and any
// command_trigger for a rule that re-fires (a fresh incident, since the prior
// one resolved) within the cooldown window — while still recording the new
// incident and pinging the lightweight incident-list refresh channel.
func TestEvaluateAlerts_CooldownSuppressesRenotification(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-cooldown"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "alert-host", Hostname: "alert-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	insertCPUMetric(t, db, hostID, 95, time.Now())

	warn := 50.0
	rule := &models.AlertRule{
		SourceType: "agent", HostID: &hostID, Metric: "cpu", Operator: ">",
		ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{Channels: []string{"browser"}, Cooldown: 300},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := &config.Config{}
	disp := dispatch.New(db)
	pusher := &stubPusher{}

	// First fire: LastFired is nil, so cooldown never applies — both the
	// incident-list refresh and the "new_alert" browser broadcast go out.
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)
	if pusher.count != 2 {
		t.Fatalf("first fire: pusher.count = %d, want 2 (list refresh + new_alert broadcast)", pusher.count)
	}

	// Recover (resolves the incident), then re-breach immediately — a
	// flapping rule re-firing well inside the 300s cooldown window.
	insertCPUMetric(t, db, hostID, 10, time.Now().Add(1*time.Second))
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)
	insertCPUMetric(t, db, hostID, 95, time.Now().Add(2*time.Second))

	countBeforeRefire := pusher.count
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err != nil {
		t.Fatalf("expected a new incident to be recorded even during cooldown, got error: %v", err)
	}
	if got := pusher.count - countBeforeRefire; got != 1 {
		t.Errorf("re-fire within cooldown: pusher.count increased by %d, want 1 (list refresh only, loud notification suppressed)", got)
	}
}

// TestEvaluateAlerts_LinksCommandTriggerToIncident checks that a rule with a
// CommandTrigger gets its dispatched command linked back onto the incident
// (AlertIncident.CommandID), and that the dispatch itself is audit-linked.
func TestEvaluateAlerts_LinksCommandTriggerToIncident(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-command-trigger"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "alert-host", Hostname: "alert-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	insertCPUMetric(t, db, hostID, 95, time.Now())

	warn := 50.0
	rule := &models.AlertRule{
		SourceType: "agent", HostID: &hostID, Metric: "cpu", Operator: ">",
		ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{
			Channels:       []string{"browser"},
			CommandTrigger: &models.CommandTrigger{Module: "processes", Action: "list"},
		},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	alerts.EvaluateAlerts(ctx, db, &config.Config{}, dispatch.New(db), &stubPusher{}, nil)

	inc, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected an open incident, got error: %v", err)
	}
	if inc.CommandID == nil || *inc.CommandID == "" {
		t.Fatal("expected the incident to be linked to a dispatched command, CommandID is nil")
	}

	cmd, err := db.GetRemoteCommandByID(ctx, *inc.CommandID)
	if err != nil || cmd == nil {
		t.Fatalf("expected the linked command to exist: %v", err)
	}
	if cmd.HostID != hostID || cmd.Module != "processes" || cmd.Action != "list" {
		t.Errorf("linked command = %+v, want host=%s module=processes action=list", cmd, hostID)
	}
	if cmd.AuditLogID == nil {
		t.Error("expected the command_trigger dispatch to be linked to an audit log entry")
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

	alerts.EvaluateAlerts(ctx, db, &config.Config{}, dispatch.New(db), &stubPusher{}, nil)

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err == nil {
		t.Error("did not expect an incident for a metric below the threshold")
	}
}
