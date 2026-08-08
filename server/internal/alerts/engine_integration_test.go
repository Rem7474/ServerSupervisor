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

// TestEvaluateAlerts_SuppressesNewIncidentDuringMaintenance ensures a host
// covered by an active maintenance window never opens a new incident, even
// with a metric well past the threshold — the core behavior the maintenance
// windows feature exists for (ROADMAP.md item #2).
func TestEvaluateAlerts_SuppressesNewIncidentDuringMaintenance(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-maintenance-1"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "alert-host", Hostname: "alert-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	insertCPUMetric(t, db, hostID, 95, time.Now())

	now := time.Now()
	if _, err := db.CreateMaintenanceWindow(ctx, models.MaintenanceWindow{
		HostID: &hostID, Reason: "planned upgrade",
		StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour), CreatedBy: "tester",
	}); err != nil {
		t.Fatalf("create maintenance window: %v", err)
	}

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
		t.Error("did not expect an incident for a host in an active maintenance window")
	}
}

// TestEvaluateAlerts_SilentlyResolvesIncidentWhenMaintenanceStarts covers the
// other half: an incident already open when a maintenance window starts must
// be closed on the next tick, same as a disabled rule.
func TestEvaluateAlerts_SilentlyResolvesIncidentWhenMaintenanceStarts(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-maintenance-2"
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
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := &config.Config{}
	disp := dispatch.New(db)

	// First tick: incident opens normally, no maintenance window yet.
	alerts.EvaluateAlerts(ctx, db, cfg, disp, &stubPusher{}, nil)
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err != nil {
		t.Fatalf("expected an open incident before maintenance started, got error: %v", err)
	}

	now := time.Now()
	if _, err := db.CreateMaintenanceWindow(ctx, models.MaintenanceWindow{
		HostID: &hostID, Reason: "planned upgrade",
		StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour), CreatedBy: "tester",
	}); err != nil {
		t.Fatalf("create maintenance window: %v", err)
	}

	// Metric still breaching — only the maintenance window changed.
	insertCPUMetric(t, db, hostID, 95, now.Add(time.Second))
	pusher := &stubPusher{}
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)

	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID); err == nil {
		t.Error("expected the incident to be silently resolved once the host entered maintenance")
	}
	if pusher.count != 1 {
		t.Errorf("pusher.count = %d, want 1 (list refresh ping only, no loud re-fire)", pusher.count)
	}
}

// TestEvaluateAlerts_EscalatesUnacknowledgedIncident covers the escalation
// half of ROADMAP.md item #3: an open, unacknowledged incident whose
// EscalateAfterMinutes has elapsed since its last notification gets
// re-notified and its last_escalated_at stamped, without opening a second
// incident. Real time can't be waited out in a unit test, so "elapsed" is
// simulated by backdating last_escalated_at directly via
// UpdateAlertIncidentLastEscalated (the exact field the engine itself reads
// to decide whether to escalate).
func TestEvaluateAlerts_EscalatesUnacknowledgedIncident(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-escalate-1"
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
		Actions: models.AlertActions{Channels: []string{"browser"}, EscalateAfterMinutes: 5},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := &config.Config{}
	disp := dispatch.New(db)

	// First tick: incident opens, no escalation yet (last_escalated_at nil,
	// triggered_at is "now").
	alerts.EvaluateAlerts(ctx, db, cfg, disp, &stubPusher{}, nil)
	inc, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected an open incident, got error: %v", err)
	}
	if inc.LastEscalatedAt != nil {
		t.Fatalf("did not expect an escalation stamp on the very first fire, got %v", *inc.LastEscalatedAt)
	}

	// Simulate "5+ minutes since the last notification" by backdating
	// last_escalated_at, then re-breach so the incident stays open.
	if err := db.UpdateAlertIncidentLastEscalated(ctx, inc.ID, time.Now().Add(-10*time.Minute)); err != nil {
		t.Fatalf("backdate last_escalated_at: %v", err)
	}
	insertCPUMetric(t, db, hostID, 95, time.Now().Add(time.Second))

	pusher := &stubPusher{}
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)

	escalated, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected the incident still open after escalation, got error: %v", err)
	}
	if escalated.ID != inc.ID {
		t.Fatalf("escalation must re-notify the existing incident (id %d), not open a new one (got id %d)", inc.ID, escalated.ID)
	}
	if escalated.LastEscalatedAt == nil || !escalated.LastEscalatedAt.After(inc.TriggeredAt) {
		t.Fatalf("expected last_escalated_at to be stamped to ~now, got %v", escalated.LastEscalatedAt)
	}
	if pusher.count == 0 {
		t.Error("expected the escalation to broadcast (list refresh + browser toast), pusher.count = 0")
	}
}

// TestEvaluateAlerts_AcknowledgedIncidentDoesNotEscalate ensures
// AcknowledgeIncident actually stops escalation, not just the UI badge — the
// entire point of tying escalation to acknowledgment.
func TestEvaluateAlerts_AcknowledgedIncidentDoesNotEscalate(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-escalate-2"
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
		Actions: models.AlertActions{Channels: []string{"browser"}, EscalateAfterMinutes: 5},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	cfg := &config.Config{}
	disp := dispatch.New(db)

	alerts.EvaluateAlerts(ctx, db, cfg, disp, &stubPusher{}, nil)
	inc, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected an open incident, got error: %v", err)
	}

	if err := db.AcknowledgeAlertIncident(ctx, inc.ID, "tester"); err != nil {
		t.Fatalf("acknowledge incident: %v", err)
	}
	if err := db.UpdateAlertIncidentLastEscalated(ctx, inc.ID, time.Now().Add(-10*time.Minute)); err != nil {
		t.Fatalf("backdate last_escalated_at: %v", err)
	}
	insertCPUMetric(t, db, hostID, 95, time.Now().Add(time.Second))

	alerts.EvaluateAlerts(ctx, db, cfg, disp, &stubPusher{}, nil)

	after, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("expected the incident still open, got error: %v", err)
	}
	if after.LastEscalatedAt == nil || !after.LastEscalatedAt.Before(time.Now().Add(-5*time.Minute)) {
		t.Errorf("expected last_escalated_at to stay untouched (still backdated) for an acknowledged incident, got %v", after.LastEscalatedAt)
	}
}

// TestBuildDockerTestTargets_MultiContainerScope confirms a "specific
// containers" scope evaluates every selected container, not just one — the
// alert engine used to only ever build a target for a single ContainerID.
func TestBuildDockerTestTargets_MultiContainerScope(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-docker-1"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "docker-host", Hostname: "docker-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	containers := []models.DockerContainer{
		{ID: "c-web", ContainerID: "web123", Name: "web", Image: "nginx", ImageTag: "1.27", State: "running"},
		{ID: "c-db", ContainerID: "db123", Name: "db", Image: "postgres", ImageTag: "16", State: "running"},
	}
	if err := db.UpsertDockerContainers(ctx, hostID, containers); err != nil {
		t.Fatalf("seed containers: %v", err)
	}

	rule := models.AlertRule{
		SourceType: "docker",
		Metric:     "docker_container_state",
		DockerScope: &models.DockerMetricScope{
			ScopeMode:    "container",
			HostID:       hostID,
			ContainerIDs: []string{"c-web", "c-db"},
		},
	}

	targets := alerts.BuildDockerTestTargets(ctx, db, rule)
	if len(targets) != 2 {
		t.Fatalf("targets = %d, want 2 (one per selected container): %+v", len(targets), targets)
	}
	gotIDs := map[string]bool{}
	for _, tg := range targets {
		gotIDs[tg.ID] = true
	}
	if !gotIDs["docker:container:c-web"] || !gotIDs["docker:container:c-db"] {
		t.Errorf("targets = %+v, want one for each of c-web and c-db", targets)
	}
}

// TestBuildDockerTestTargets_LegacySingleContainerScope confirms a rule saved
// before multi-select existed (ContainerID only, ContainerIDs empty) still
// evaluates its one container unchanged.
func TestBuildDockerTestTargets_LegacySingleContainerScope(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "alert-host-docker-2"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "docker-host", Hostname: "docker-host", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	if err := db.UpsertDockerContainers(ctx, hostID, []models.DockerContainer{
		{ID: "c-web", ContainerID: "web123", Name: "web", Image: "nginx", ImageTag: "1.27", State: "running"},
	}); err != nil {
		t.Fatalf("seed container: %v", err)
	}

	rule := models.AlertRule{
		SourceType: "docker",
		Metric:     "docker_container_state",
		DockerScope: &models.DockerMetricScope{
			ScopeMode:   "container",
			HostID:      hostID,
			ContainerID: "c-web",
		},
	}

	targets := alerts.BuildDockerTestTargets(ctx, db, rule)
	if len(targets) != 1 || targets[0].ID != "docker:container:c-web" {
		t.Fatalf("targets = %+v, want exactly one for docker:container:c-web", targets)
	}
}
