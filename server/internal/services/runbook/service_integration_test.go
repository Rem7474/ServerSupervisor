package runbook_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/services/runbook"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestRunbook_FullLifecycle exercises the real SQL in db_runbooks.go end to
// end against a real Postgres — the unit tests in service_test.go cover the
// state-machine logic against a fake Repository, which can't catch a wrong
// column name or a type mismatch in the actual queries.
func TestRunbook_FullLifecycle(t *testing.T) {
	db, _ := testutil.NewPostgresDBWithConfig(t)
	ctx := context.Background()

	for _, id := range []string{"rb-host-1", "rb-host-2"} {
		if err := db.RegisterHost(ctx, &models.Host{
			ID: id, Name: id, Hostname: id, Status: "online", LastSeen: time.Now(),
		}); err != nil {
			t.Fatalf("register host %s: %v", id, err)
		}
	}

	disp := dispatch.New(db)
	svc := runbook.NewService(db, disp)

	rb, err := svc.Create(ctx, models.RunbookCreate{
		Name: "Integration test runbook",
		Steps: []models.RunbookStepCreate{
			{HostID: "rb-host-1", Module: "docker", Action: "stop", Target: "app"},
			{HostID: "rb-host-2", Module: "systemd", Action: "restart", Target: "nginx"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(rb.Steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(rb.Steps))
	}

	exec, err := svc.Run(ctx, rb.ID, "tester")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exec.Status != "running" {
		t.Fatalf("status = %q, want running", exec.Status)
	}

	detail, err := svc.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if len(detail.Steps) != 2 {
		t.Fatalf("got %d execution steps, want 2", len(detail.Steps))
	}
	if detail.Steps[0].CommandID == nil {
		t.Fatal("expected step 0 to have a dispatched command")
	}
	if detail.Steps[1].CommandID != nil {
		t.Fatal("expected step 1 to not be dispatched yet")
	}

	svc.NotifyComplete(ctx, *detail.Steps[0].CommandID, "completed")

	afterStep1, err := svc.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution after step 1: %v", err)
	}
	if afterStep1.Status != "running" {
		t.Fatalf("status after step 1 = %q, want running", afterStep1.Status)
	}
	if afterStep1.Steps[1].CommandID == nil {
		t.Fatal("expected step 2 to now have a dispatched command")
	}

	svc.NotifyComplete(ctx, *afterStep1.Steps[1].CommandID, "completed")

	final, err := svc.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution final: %v", err)
	}
	if final.Status != "completed" {
		t.Fatalf("final status = %q, want completed", final.Status)
	}

	execs, err := svc.ListExecutions(ctx, rb.ID, 10)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("got %d executions, want 1", len(execs))
	}

	if err := svc.Delete(ctx, rb.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, rb.ID); err == nil {
		t.Fatal("expected the runbook to be gone after Delete")
	}
}

func TestRunbook_FailureStopsExecutionByDefault(t *testing.T) {
	db, _ := testutil.NewPostgresDBWithConfig(t)
	ctx := context.Background()

	if err := db.RegisterHost(ctx, &models.Host{
		ID: "rb-fail-host", Name: "h", Hostname: "h", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	disp := dispatch.New(db)
	svc := runbook.NewService(db, disp)

	rb, err := svc.Create(ctx, models.RunbookCreate{
		Name: "Fails fast",
		Steps: []models.RunbookStepCreate{
			{HostID: "rb-fail-host", Module: "docker", Action: "stop", Target: "app"},
			{HostID: "rb-fail-host", Module: "docker", Action: "start", Target: "app"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	exec, err := svc.Run(ctx, rb.ID, "tester")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	detail, err := svc.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}

	svc.NotifyComplete(ctx, *detail.Steps[0].CommandID, "failed")

	final, err := svc.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if final.Status != "failed" {
		t.Fatalf("status = %q, want failed", final.Status)
	}
	if final.Steps[1].CommandID != nil {
		t.Fatal("step 2 must not have been dispatched after step 1 failed without continue_on_failure")
	}
}
