package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// The storage-derived upsert exists to fill in guests a multi-VM vzdump job
// swallowed (one task, no vmid), without trampling the richer task-derived
// rows — those carry a UPID the UI links a log to. Its precedence rule and its
// node resolution are the parts worth pinning.
func TestUpsertProxmoxBackupRunFromStorage(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	connID, err := db.CreateProxmoxConnection(ctx, baseConnRequest("backup-runs"))
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	older := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	runFor := func(vmid int) models.ProxmoxBackupRun {
		t.Helper()
		runs, err := db.ListProxmoxBackupRuns(ctx, connID)
		if err != nil {
			t.Fatalf("list backup runs: %v", err)
		}
		for _, r := range runs {
			if r.VMID == vmid {
				return r
			}
		}
		t.Fatalf("no run for vmid %d in %+v", vmid, runs)
		return models.ProxmoxBackupRun{}
	}

	t.Run("inserts a guest that has no run yet", func(t *testing.T) {
		if err := db.UpsertProxmoxBackupRunFromStorage(ctx, connID, "pve1", 300, "OK", newer); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		got := runFor(300)
		if got.TaskUPID != "" {
			t.Errorf("task_upid = %q, want empty (no per-guest task exists)", got.TaskUPID)
		}
		if got.EndTime == nil || !got.EndTime.Equal(newer) {
			t.Errorf("end_time = %v, want %v", got.EndTime, newer)
		}
	})

	t.Run("does not overwrite a more recent task-derived row", func(t *testing.T) {
		if err := db.UpsertProxmoxBackupRun(ctx, connID, "pve1", 200, "UPID:pve1:VM200", "job errors", &older, &newer, "job errors"); err != nil {
			t.Fatalf("seed task run: %v", err)
		}
		// A retained volume older than the task result must lose.
		if err := db.UpsertProxmoxBackupRunFromStorage(ctx, connID, "pve1", 200, "OK", older); err != nil {
			t.Fatalf("upsert: %v", err)
		}

		got := runFor(200)
		if got.TaskUPID != "UPID:pve1:VM200" {
			t.Errorf("task link lost: task_upid = %q", got.TaskUPID)
		}
		if got.Status != "job errors" {
			t.Errorf("status = %q, want the task's own outcome", got.Status)
		}
	})

	t.Run("takes over when the volume is newer than the recorded task", func(t *testing.T) {
		evenNewer := newer.Add(48 * time.Hour)
		if err := db.UpsertProxmoxBackupRunFromStorage(ctx, connID, "pve1", 200, "OK", evenNewer); err != nil {
			t.Fatalf("upsert: %v", err)
		}

		got := runFor(200)
		if got.EndTime == nil || !got.EndTime.Equal(evenNewer) {
			t.Errorf("end_time = %v, want %v", got.EndTime, evenNewer)
		}
		// The stale UPID pointed at an older backup; keeping it would link the
		// log of a run that is no longer the latest.
		if got.TaskUPID != "" {
			t.Errorf("task_upid = %q, want it cleared", got.TaskUPID)
		}
	})

	t.Run("resolves the owning node rather than the storage's listing node", func(t *testing.T) {
		// Backup storages are usually shared, so the node that listed the volume
		// is not necessarily the one running the guest — and the UI filters this
		// table by node.
		if err := db.UpsertProxmoxGuest(ctx, connID, "pve2", "vm", 400, "db-01", "running", 2, 0, 0, 0, 0, 0, 0, ""); err != nil {
			t.Fatalf("seed guest: %v", err)
		}
		if err := db.UpsertProxmoxBackupRunFromStorage(ctx, connID, "pve1", 400, "OK", newer); err != nil {
			t.Fatalf("upsert: %v", err)
		}

		if got := runFor(400); got.NodeName != "pve2" {
			t.Errorf("node_name = %q, want the guest's own node pve2", got.NodeName)
		}
	})
}
