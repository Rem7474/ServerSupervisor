package dispatch

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/testutil"
)

// seedHost inserts a minimal host row so remote_commands' host_id foreign key
// (ON DELETE CASCADE to hosts.id) is satisfiable.
func seedHost(t *testing.T, db *database.DB, id string) {
	t.Helper()
	testutil.MustQuery(t, db,
		`INSERT INTO hosts (id, name, ip_address, api_key, status) VALUES ($1, $2, '10.0.0.1', 'dummyhash', 'offline')`,
		id, "dispatch-test-host")
}

func TestCreate_WithoutAudit(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedHost(t, db, "host-1")
	d := New(db)

	result, err := d.Create(context.Background(), Request{
		HostID:      "host-1",
		Module:      "docker",
		Action:      "restart",
		Target:      "my-container",
		Payload:     `{"foo":"bar"}`,
		TriggeredBy: "alice",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Command == nil {
		t.Fatal("expected a non-nil Command")
	}
	if result.Command.HostID != "host-1" || result.Command.Module != "docker" || result.Command.Action != "restart" {
		t.Errorf("unexpected command fields: %+v", result.Command)
	}
	if result.AuditLogID != nil {
		t.Errorf("AuditLogID = %v, want nil when no Audit request was supplied", *result.AuditLogID)
	}
	if result.Command.AuditLogID != nil {
		t.Errorf("Command.AuditLogID = %v, want nil", *result.Command.AuditLogID)
	}
}

func TestCreate_WithAuditLinksTheAuditLog(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedHost(t, db, "host-2")
	d := New(db)

	result, err := d.Create(context.Background(), Request{
		HostID:      "host-2",
		Module:      "apt",
		Action:      "upgrade",
		TriggeredBy: "bob",
		Audit: &AuditLogRequest{
			Username:  "bob",
			Action:    "apt_upgrade",
			HostID:    "host-2",
			IPAddress: "192.0.2.1",
			Details:   "manual upgrade",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.AuditLogID == nil {
		t.Fatal("expected a non-nil AuditLogID when an Audit request was supplied")
	}
	if result.Command.AuditLogID == nil || *result.Command.AuditLogID != *result.AuditLogID {
		t.Errorf("Command.AuditLogID = %v, want %v", result.Command.AuditLogID, *result.AuditLogID)
	}

	logs, err := db.GetAuditLogsByHost(context.Background(), "host-2", 10)
	if err != nil {
		t.Fatalf("GetAuditLogsByHost returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("got %d audit logs for host-2, want 1", len(logs))
	}
	if logs[0].Status != "pending" {
		t.Errorf("audit log status = %q, want %q (Create only marks it 'failed' on error)", logs[0].Status, "pending")
	}
}

func TestCreate_CommandFailureMarksAuditLogFailed(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	d := New(db)

	// No host row was seeded for "does-not-exist" — CreateRemoteCommand must
	// fail on the host_id foreign key, exercising the error path where Create
	// marks the just-created audit log as "failed" instead of leaving it
	// "pending" forever.
	_, err := d.Create(context.Background(), Request{
		HostID:      "does-not-exist",
		Module:      "docker",
		Action:      "restart",
		TriggeredBy: "carol",
		Audit: &AuditLogRequest{
			Username: "carol",
			Action:   "docker_restart",
			HostID:   "does-not-exist",
			Details:  "should fail",
		},
	})
	if err == nil {
		t.Fatal("expected Create to fail for a nonexistent host_id")
	}

	logs, lerr := db.GetAuditLogsByHost(context.Background(), "does-not-exist", 10)
	if lerr != nil {
		t.Fatalf("GetAuditLogsByHost returned error: %v", lerr)
	}
	if len(logs) != 1 {
		t.Fatalf("got %d audit logs, want 1", len(logs))
	}
	if logs[0].Status != "failed" {
		t.Errorf("audit log status = %q, want %q", logs[0].Status, "failed")
	}
}
