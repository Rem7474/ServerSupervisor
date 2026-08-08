package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

const testBackupHostID = "host-backup-test"

// TestCreateBackupRun_PersistsFullyPopulatedRow guards the scheduled-task
// backup path: HandleCommandCompletion's insert branch (taken when no
// TriggerBackup placeholder row exists yet — the case for a scheduler-
// dispatched restic run_backup, see server/CLAUDE.md's "Restic backups"
// section) passes a fully-populated models.BackupRun straight into
// CreateBackupRun. It must come back out with duration/volume/etc intact —
// previously the INSERT only wrote the 6 "running placeholder" columns
// (host_id/profile/command_id/triggered_by/status/started_at) and silently
// dropped the rest, so a scheduler-triggered backup always showed an empty
// "Durée"/"Volume" in the UI even though it succeeded.
func TestCreateBackupRun_PersistsFullyPopulatedRow(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	if err := db.RegisterHost(ctx, &models.Host{
		ID: testBackupHostID, Name: "backup-test", Hostname: "backup.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	cmd, err := db.CreateRemoteCommand(ctx, testBackupHostID, "restic", "run_backup", "nextcloud-full", "{}", "scheduler", nil)
	if err != nil {
		t.Fatalf("create remote command: %v", err)
	}
	commandID := cmd.ID
	started := time.Now().Add(-63 * time.Minute).Truncate(time.Second)
	finished := time.Now().Truncate(time.Second)
	durationSec := 3780
	filesDone := int64(42)
	bytesDone := int64(115_900_000_000)
	repoSize := int64(500_000_000_000)
	snapshotTime := finished
	rawSummary := `{"status":"ok","snapshot_id":"abc123"}`

	created, err := db.CreateBackupRun(ctx, models.BackupRun{
		HostID:        testBackupHostID,
		Profile:       "nextcloud-full",
		CommandID:     &commandID,
		TriggeredBy:   "scheduler",
		Status:        "ok",
		StartedAt:     started,
		FinishedAt:    &finished,
		DurationSec:   &durationSec,
		FilesDone:     &filesDone,
		BytesDone:     &bytesDone,
		RepoSizeBytes: &repoSize,
		SnapshotID:    "abc123",
		SnapshotTime:  &snapshotTime,
		RawSummary:    &rawSummary,
	})
	if err != nil {
		t.Fatalf("CreateBackupRun: %v", err)
	}

	// Re-fetch from the DB (not just the RETURNING clause) to make sure the
	// columns were actually persisted, not just echoed back in memory.
	got, err := db.GetBackupRun(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetBackupRun: %v", err)
	}

	if got.FinishedAt == nil {
		t.Error("expected finished_at to be persisted")
	}
	if got.DurationSec == nil || *got.DurationSec != durationSec {
		t.Errorf("expected duration_sec=%d to be persisted, got %v", durationSec, got.DurationSec)
	}
	if got.FilesDone == nil || *got.FilesDone != filesDone {
		t.Errorf("expected files_done=%d to be persisted, got %v", filesDone, got.FilesDone)
	}
	if got.BytesDone == nil || *got.BytesDone != bytesDone {
		t.Errorf("expected bytes_done=%d to be persisted, got %v", bytesDone, got.BytesDone)
	}
	if got.RepoSizeBytes == nil || *got.RepoSizeBytes != repoSize {
		t.Errorf("expected repo_size_bytes=%d to be persisted, got %v", repoSize, got.RepoSizeBytes)
	}
	if got.SnapshotID != "abc123" {
		t.Errorf("expected snapshot_id to be persisted, got %q", got.SnapshotID)
	}
	if got.SnapshotTime == nil {
		t.Error("expected snapshot_time to be persisted")
	}
	if got.RawSummary == nil || *got.RawSummary != rawSummary {
		t.Errorf("expected raw_summary to be persisted, got %v", got.RawSummary)
	}
}

// TestCreateBackupRun_PlaceholderRowStaysMinimal guards the other caller —
// TriggerBackup's initial "running" row (manual trigger) — still works when
// only the placeholder fields are set: everything else must come back nil,
// not zero-valued garbage.
func TestCreateBackupRun_PlaceholderRowStaysMinimal(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	if err := db.RegisterHost(ctx, &models.Host{
		ID: testBackupHostID, Name: "backup-test", Hostname: "backup.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	created, err := db.CreateBackupRun(ctx, models.BackupRun{
		HostID:      testBackupHostID,
		Profile:     "files",
		TriggeredBy: "alice",
		Status:      "running",
		StartedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateBackupRun: %v", err)
	}

	if created.Status != "running" {
		t.Errorf("expected status=running, got %q", created.Status)
	}
	if created.DurationSec != nil {
		t.Errorf("expected duration_sec=nil for a fresh placeholder row, got %v", created.DurationSec)
	}
	if created.FinishedAt != nil {
		t.Errorf("expected finished_at=nil for a fresh placeholder row, got %v", created.FinishedAt)
	}
	if created.BytesDone != nil {
		t.Errorf("expected bytes_done=nil for a fresh placeholder row, got %v", created.BytesDone)
	}
}
