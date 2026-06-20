package database_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

const testHostID = "host-cmd-test"

func seedTestHost(t *testing.T, db *database.DB) {
	t.Helper()
	if err := db.RegisterHost(context.Background(), &models.Host{
		ID: testHostID, Name: "cmd-test", Hostname: "cmd.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
}

func TestRemoteCommand_CreateAndClaimAtomically(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db)
	ctx := context.Background()

	cmd, err := db.CreateRemoteCommand(ctx, testHostID, "docker", "restart", "nginx", "{}", "alice", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cmd.ID == "" {
		t.Fatal("created command must have a UUID")
	}
	if cmd.Status != "pending" {
		t.Errorf("expected status=pending, got %q", cmd.Status)
	}

	claimed, err := db.ClaimPendingRemoteCommands(ctx, testHostID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != cmd.ID {
		t.Fatalf("claim mismatch: %+v", claimed)
	}

	// Re-claiming must yield zero — the row is now in 'running' state.
	again, err := db.ClaimPendingRemoteCommands(ctx, testHostID)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("a claimed command must not be re-claimable, got %+v", again)
	}
}

// TestRemoteCommand_ConcurrentClaimNeverDuplicates exercises the SKIP LOCKED
// path: with N agents calling Claim concurrently, a given command must end up
// in exactly one of the result sets — never zero, never two.
func TestRemoteCommand_ConcurrentClaimNeverDuplicates(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db)
	ctx := context.Background()

	const n = 10
	for i := 0; i < n; i++ {
		if _, err := db.CreateRemoteCommand(ctx, testHostID, "docker", "restart", "svc", "{}", "tester", nil); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}

	const workers = 5
	results := make([][]string, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmds, err := db.ClaimPendingRemoteCommands(ctx, testHostID)
			if err != nil {
				t.Errorf("worker %d claim: %v", idx, err)
				return
			}
			ids := make([]string, 0, len(cmds))
			for _, c := range cmds {
				ids = append(ids, c.ID)
			}
			results[idx] = ids
		}(w)
	}
	wg.Wait()

	seen := map[string]int{}
	for _, ids := range results {
		for _, id := range ids {
			seen[id]++
		}
	}
	if len(seen) != n {
		t.Errorf("expected %d distinct commands across workers, got %d", n, len(seen))
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("command %s claimed %d times — SKIP LOCKED broken", id, count)
		}
	}
}

func TestRemoteCommand_StatusTransition(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db)
	ctx := context.Background()

	cmd, err := db.CreateRemoteCommand(ctx, testHostID, "apt", "update", "", "{}", "tester", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.UpdateRemoteCommandStatus(ctx, cmd.ID, "completed", "ok"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := db.GetRemoteCommandByID(ctx, cmd.ID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("status=%q, want completed", got.Status)
	}
	if got.Output != "ok" {
		t.Errorf("output=%q, want ok", got.Output)
	}
	if got.EndedAt == nil {
		t.Error("ended_at must be set on terminal status")
	}
}

func TestCleanupStalledCommands_FailsOldPendingAndCascadesToAudit(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db)
	ctx := context.Background()

	// Insert audit log row first since remote_commands.audit_log_id FK SET NULL.
	auditID, err := db.CreateAuditLog(ctx, "tester", "test", testHostID, "127.0.0.1", "test", "pending")
	if err != nil {
		t.Fatalf("create audit: %v", err)
	}

	cmd, err := db.CreateRemoteCommand(ctx, testHostID, "docker", "stop", "svc", "{}", "tester", &auditID)
	if err != nil {
		t.Fatalf("create cmd: %v", err)
	}

	// Force the row to look old enough for cleanup to consider it stalled.
	testutil.MustQuery(t, db,
		`UPDATE remote_commands SET created_at = NOW() - INTERVAL '20 minutes' WHERE id = $1`, cmd.ID)

	// Trigger cleanup with a 10-minute threshold — our row is 20 minutes old.
	if err := db.CleanupStalledCommands(ctx, 10); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	got, err := db.GetRemoteCommandByID(ctx, cmd.ID)
	if err != nil {
		t.Fatalf("fetch cmd: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("cleanup must mark stalled cmd as failed, got %q", got.Status)
	}

	// Audit log must be reconciled as failed too.
	row := db.QueryRow(ctx, `SELECT status FROM audit_logs WHERE id = $1`, auditID)
	var status string
	if err := row.Scan(&status); err != nil {
		t.Fatalf("scan audit status: %v", err)
	}
	if status != "failed" {
		t.Errorf("cleanup did not cascade to audit_log: status=%q", status)
	}

	_ = time.Now() // keeps the time import used (we may add timed assertions later).
}

// TestCleanupStalledCommands_ActivityHeartbeatPreventsReap guards the activity-based
// reaping: a running command whose row is older than the threshold must NOT be failed
// as long as its activity was refreshed recently (the per-report heartbeat). Without a
// recent heartbeat the same row is reaped — proving the COALESCE keys off activity.
func TestCleanupStalledCommands_ActivityHeartbeatPreventsReap(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db)
	ctx := context.Background()

	cmd, err := db.CreateRemoteCommand(ctx, testHostID, "apt", "update", "", "{}", "tester", nil)
	if err != nil {
		t.Fatalf("create cmd: %v", err)
	}
	if _, err := db.ClaimPendingRemoteCommands(ctx, testHostID); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Simulate a command that started 20 minutes ago and has been streaming output:
	// created/started are old, but the heartbeat bumped activity to NOW().
	testutil.MustQuery(t, db,
		`UPDATE remote_commands
		 SET created_at = NOW() - INTERVAL '20 minutes',
		     started_at = NOW() - INTERVAL '20 minutes'
		 WHERE id = $1`, cmd.ID)
	if err := db.TouchRunningCommandsActivity(ctx, testHostID); err != nil {
		t.Fatalf("touch activity: %v", err)
	}

	if err := db.CleanupHostStalledCommands(ctx, testHostID, 10); err != nil {
		t.Fatalf("cleanup (active): %v", err)
	}
	got, err := db.GetRemoteCommandByID(ctx, cmd.ID)
	if err != nil {
		t.Fatalf("fetch cmd: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("recently-active command must survive cleanup, got %q", got.Status)
	}

	// Now let the heartbeat go stale (last activity 20 minutes ago) → must be reaped.
	testutil.MustQuery(t, db,
		`UPDATE remote_commands SET last_activity_at = NOW() - INTERVAL '20 minutes' WHERE id = $1`, cmd.ID)
	if err := db.CleanupHostStalledCommands(ctx, testHostID, 10); err != nil {
		t.Fatalf("cleanup (stale): %v", err)
	}
	got, err = db.GetRemoteCommandByID(ctx, cmd.ID)
	if err != nil {
		t.Fatalf("fetch cmd: %v", err)
	}
	if got.Status != "failed" {
		t.Errorf("inactive command must be reaped, got %q", got.Status)
	}
}

func TestFailRunningCommandsOnAgentReconnect_OnlyTouchesGivenHost(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	seedTestHost(t, db) // host-cmd-test
	ctx := context.Background()
	otherID := "host-other"
	if err := db.RegisterHost(ctx, &models.Host{ID: otherID, Name: "other", Hostname: "other.local", Status: "online"}); err != nil {
		t.Fatalf("register other: %v", err)
	}

	cmdA, _ := db.CreateRemoteCommand(ctx, testHostID, "docker", "stop", "svc", "{}", "tester", nil)
	cmdB, _ := db.CreateRemoteCommand(ctx, otherID, "docker", "stop", "svc", "{}", "tester", nil)

	// Both move to 'running' via the normal claim path.
	if _, err := db.ClaimPendingRemoteCommands(ctx, testHostID); err != nil {
		t.Fatalf("claim A: %v", err)
	}
	if _, err := db.ClaimPendingRemoteCommands(ctx, otherID); err != nil {
		t.Fatalf("claim B: %v", err)
	}

	// Only the first host reconnects.
	if err := db.FailRunningCommandsOnAgentReconnect(ctx, testHostID); err != nil {
		t.Fatalf("fail running: %v", err)
	}

	a, _ := db.GetRemoteCommandByID(ctx, cmdA.ID)
	b, _ := db.GetRemoteCommandByID(ctx, cmdB.ID)
	if a.Status != "failed" {
		t.Errorf("cmdA must be failed, got %q", a.Status)
	}
	if b.Status != "running" {
		t.Errorf("cmdB on unrelated host must remain running, got %q", b.Status)
	}
}
