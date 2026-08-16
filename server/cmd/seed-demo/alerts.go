package main

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

// seedAlerts upserts two alert rules by their fixed Name (find-or-update,
// same pattern as demoHost) and reseeds their incidents from scratch each
// run (delete-by-rule-id then insert with explicit triggered_at/resolved_at,
// since CreateAlertIncident/ResolveAlertIncident always stamp NOW()) — one
// still-open incident (the "unresolved alert" edge case) and one resolved
// incident for history.
func seedAlerts(ctx context.Context, db *database.DB) error {
	existing, err := db.GetAlertRules(ctx)
	if err != nil {
		return err
	}
	byName := map[string]int64{}
	for _, r := range existing {
		if r.Name != nil {
			byName[*r.Name] = r.ID
		}
	}

	cpuRule := &models.AlertRule{
		Name: strPtr("Demo - CPU élevé (app-02)"), SourceType: models.AlertSourceAgent,
		HostID: strPtr("demo-app-02"), Metric: "cpu", Operator: ">",
		ThresholdWarn: f64Ptr(80), ThresholdCrit: f64Ptr(90), DurationSeconds: 300,
		Actions: models.AlertActions{Channels: []string{"browser"}}, Enabled: true,
	}
	diskRule := &models.AlertRule{
		Name: strPtr("Demo - Disque plein (db-01)"), SourceType: models.AlertSourceAgent,
		HostID: strPtr("demo-db-01"), Metric: "disk", Operator: ">",
		ThresholdWarn: f64Ptr(85), ThresholdCrit: f64Ptr(90), DurationSeconds: 300,
		Actions: models.AlertActions{Channels: []string{"browser"}}, Enabled: true,
	}

	for _, rule := range []*models.AlertRule{cpuRule, diskRule} {
		if id, ok := byName[*rule.Name]; ok {
			rule.ID = id
			if err := db.UpdateAlertRule(ctx, rule); err != nil {
				return err
			}
		} else if err := db.CreateAlertRule(ctx, rule); err != nil {
			return err
		}
	}

	if _, err := db.Exec(ctx, `DELETE FROM alert_incidents WHERE rule_id = ANY($1)`,
		pq.Array([]int64{cpuRule.ID, diskRule.ID})); err != nil {
		return err
	}

	// Still-open incident — the "unresolved alert" edge case.
	if _, err := db.Exec(ctx,
		`INSERT INTO alert_incidents (rule_id, host_id, value, severity, triggered_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		cpuRule.ID, "demo-app-02", 92.3, "crit", anchor.Add(-25*time.Minute),
	); err != nil {
		return err
	}

	// Resolved incident — alert history.
	if _, err := db.Exec(ctx,
		`INSERT INTO alert_incidents (rule_id, host_id, value, severity, triggered_at, resolved_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		diskRule.ID, "demo-db-01", 91.0, "warn", anchor.Add(-26*time.Hour), anchor.Add(-24*time.Hour),
	); err != nil {
		return err
	}

	return nil
}
