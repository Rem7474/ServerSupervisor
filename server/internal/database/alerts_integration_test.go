package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// Integration test for the alert-rule + alert-incident lifecycle. These
// queries lean on Postgres-specific bits (JSONB columns, UNIQUE constraints,
// RETURNING) so sqlmock would not catch a real regression — only a live
// Postgres can.
func TestAlertRule_CRUDRoundTrip(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "host-alpha"
	if err := db.RegisterHost(ctx, &models.Host{ID: hostID, Name: "alpha", Hostname: "alpha.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	thresholdWarn := 80.0
	thresholdCrit := 95.0
	rule := &models.AlertRule{
		SourceType:      models.AlertSourceAgent,
		HostID:          &hostID,
		Metric:          "cpu_percent",
		Operator:        ">",
		ThresholdWarn:   &thresholdWarn,
		ThresholdCrit:   &thresholdCrit,
		DurationSeconds: 60,
		Enabled:         true,
		Actions: models.AlertActions{
			Channels: []string{"browser"},
		},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("rule.ID was not populated by RETURNING clause")
	}

	rules, err := db.GetAlertRules(ctx)
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	var found *models.AlertRule
	for i := range rules {
		if rules[i].ID == rule.ID {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		t.Fatal("created rule not returned by GetAlertRules")
	}
	if found.Metric != "cpu_percent" || found.Operator != ">" {
		t.Errorf("rule round-trip lost data: metric=%q operator=%q", found.Metric, found.Operator)
	}
	if len(found.Actions.Channels) != 1 || found.Actions.Channels[0] != "browser" {
		t.Errorf("actions.channels not preserved across JSONB round-trip: %+v", found.Actions)
	}

	if err := db.DeleteAlertRule(ctx, rule.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	rules, _ = db.GetAlertRules(ctx)
	for _, r := range rules {
		if r.ID == rule.ID {
			t.Fatal("rule still present after delete")
		}
	}
}

func TestAlertIncident_FireAndResolveLifecycle(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "host-beta"
	if err := db.RegisterHost(ctx, &models.Host{ID: hostID, Name: "beta", Hostname: "beta.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	thresholdWarn := 80.0
	rule := &models.AlertRule{
		SourceType:    models.AlertSourceAgent,
		HostID:        &hostID,
		Metric:        "cpu_percent",
		Operator:      ">",
		ThresholdWarn: &thresholdWarn,
		Enabled:       true,
		Actions:       models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// No incident exists yet.
	if open, _ := db.GetOpenAlertIncident(ctx, rule.ID, hostID); open != nil {
		t.Fatalf("expected no open incident, got %+v", open)
	}

	// Fire an incident.
	incidentID, err := db.CreateAlertIncident(ctx, rule.ID, hostID, 91.5, "warn")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	// It should now be returned as open.
	open, err := db.GetOpenAlertIncident(ctx, rule.ID, hostID)
	if err != nil {
		t.Fatalf("get open: %v", err)
	}
	if open == nil || open.ID != incidentID {
		t.Fatalf("open incident mismatch, got %+v", open)
	}
	if open.Severity != "warn" {
		t.Errorf("severity not persisted: %q", open.Severity)
	}

	// Resolve it.
	if err := db.ResolveAlertIncident(ctx, incidentID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if open, _ := db.GetOpenAlertIncident(ctx, rule.ID, hostID); open != nil {
		t.Fatalf("incident still open after resolve: %+v", open)
	}

	// Firing again should produce a fresh incident with a new ID.
	newID, err := db.CreateAlertIncident(ctx, rule.ID, hostID, 88.0, "warn")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if newID == incidentID {
		t.Fatal("re-fired incident must have a new ID")
	}
}

func TestAlertIncident_ResolveAllOpenByRule(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostIDs := []string{"host-c1", "host-c2", "host-c3"}
	for _, id := range hostIDs {
		if err := db.RegisterHost(ctx, &models.Host{ID: id, Name: id, Hostname: id + ".local", Status: "online"}); err != nil {
			t.Fatalf("register host %s: %v", id, err)
		}
	}

	w := 50.0
	rule := &models.AlertRule{
		SourceType:    models.AlertSourceAgent,
		Metric:        "memory_percent",
		Operator:      ">",
		ThresholdWarn: &w,
		Enabled:       true,
		Actions:       models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// Fire one incident per host.
	for _, id := range hostIDs {
		if _, err := db.CreateAlertIncident(ctx, rule.ID, id, 70.0, "warn"); err != nil {
			t.Fatalf("create incident for %s: %v", id, err)
		}
	}

	open, err := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != len(hostIDs) {
		t.Fatalf("expected %d open, got %d", len(hostIDs), len(open))
	}

	// Bulk-resolve.
	n, err := db.ResolveOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("bulk resolve: %v", err)
	}
	if int(n) != len(hostIDs) {
		t.Errorf("bulk resolve count: expected %d, got %d", len(hostIDs), n)
	}

	open, _ = db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if len(open) != 0 {
		t.Fatalf("expected 0 open after bulk resolve, got %d", len(open))
	}
}
