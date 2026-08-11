package main

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// demoAuditActor is the username attached to every seeded audit_logs row —
// deliberately distinct from the ADMIN_USER login ("demo") so a re-seed can
// precisely delete only what it created without touching real activity a
// human generated while poking around the demo UI under the admin account.
const demoAuditActor = "demo-team"

type demoAuditEntry struct {
	Action  string
	HostID  string
	Details string
	Status  string
	Ago     time.Duration
}

var demoAuditEntries = []demoAuditEntry{
	{Action: "apt_upgrade", HostID: "demo-web-01", Details: `{"packages":15}`, Status: "completed", Ago: 4 * time.Hour},
	{Action: "docker_restart", HostID: "demo-db-01", Details: `{"container":"demo-db-01-redis"}`, Status: "completed", Ago: 6 * time.Hour},
	{Action: "alert_rule_created", HostID: "demo-app-02", Details: `{"rule":"Demo - CPU élevé (app-02)"}`, Status: "completed", Ago: 26 * time.Hour},
	{Action: "alert_incident_resolved", HostID: "demo-db-01", Details: `{"rule":"Demo - Disque plein (db-01)"}`, Status: "completed", Ago: 24 * time.Hour},
	{Action: "unblock_ip", HostID: "", Details: `{"ip":"203.0.113.42"}`, Status: "completed", Ago: 10 * time.Hour},
	{Action: "update_settings", HostID: "", Details: `{"field":"metrics_retention_days"}`, Status: "completed", Ago: 30 * time.Hour},
	{Action: "cleanup_metrics", HostID: "", Details: `{"deleted_rows":18342}`, Status: "completed", Ago: 20 * time.Hour},
	{Action: "docker_compose_up", HostID: "demo-web-01", Details: `{"project":"web-stack"}`, Status: "completed", Ago: 12 * time.Hour},
	{Action: "docker_stop", HostID: "demo-app-02", Details: `{"container":"demo-app-02-worker"}`, Status: "failed", Ago: 45 * time.Minute},
	{Action: "journalctl", HostID: "demo-db-01", Details: `{"unit":"postgresql"}`, Status: "completed", Ago: 2 * time.Hour},
	{Action: "webhook_trigger", HostID: "demo-web-01", Details: `{"repo":"acme/frontend"}`, Status: "completed", Ago: 15 * time.Hour},
	{Action: "agent_update", HostID: "demo-app-02", Details: `{"version":"1.8.2"}`, Status: "completed", Ago: 48 * time.Hour},
	{Action: "host_created", HostID: "demo-worker-01", Details: `{}`, Status: "completed", Ago: 96 * time.Hour},
	{Action: "user_created", HostID: "", Details: `{"username":"j.martin"}`, Status: "completed", Ago: 72 * time.Hour},
	{Action: "scheduled_task_created", HostID: "demo-web-01", Details: `{"task":"Demo - Nightly APT upgrade"}`, Status: "completed", Ago: 50 * time.Hour},
	{Action: "apt_update", HostID: "demo-app-02", Details: `{"pending":15}`, Status: "completed", Ago: 8 * time.Hour},
	{Action: "docker_restart", HostID: "demo-app-02", Details: `{"container":"demo-app-02-worker"}`, Status: "failed", Ago: 20 * time.Minute},
	{Action: "alert_incident_acknowledged", HostID: "demo-app-02", Details: `{"rule":"Demo - CPU élevé (app-02)"}`, Status: "completed", Ago: 15 * time.Minute},
	{Action: "cleanup_audit_logs", HostID: "", Details: `{"deleted_rows":540}`, Status: "completed", Ago: 60 * time.Hour},
	{Action: "docker_stop", HostID: "demo-worker-01", Details: `{"container":"demo-worker-01-batch"}`, Status: "completed", Ago: 6 * time.Hour},
}

// seedAudit deletes every previously seeded row (scoped by demoAuditActor)
// then reinserts with explicit created_at offsets — CreateAuditLog always
// stamps NOW(), so a raw insert is used here to get a realistic trailing
// history instead of 20 rows all timestamped "just now".
func seedAudit(ctx context.Context, db *database.DB) error {
	if _, err := db.Exec(ctx, `DELETE FROM audit_logs WHERE username = $1`, demoAuditActor); err != nil {
		return err
	}
	for _, e := range demoAuditEntries {
		if _, err := db.Exec(ctx,
			`INSERT INTO audit_logs (username, action, host_id, ip_address, details, status, category, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			demoAuditActor, e.Action, e.HostID, "10.0.0.5", e.Details, e.Status,
			models.CategorizeAuditAction(e.Action), anchor.Add(-e.Ago),
		); err != nil {
			return err
		}
	}
	return nil
}
