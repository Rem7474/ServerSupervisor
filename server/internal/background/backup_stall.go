package background

import (
	"context"
	"time"

	backupsvc "github.com/serversupervisor/server/internal/services/backup"
)

// backupStallCheckInterval is how often the scan runs. staleAfterMinutes (the
// per-run threshold) is intentionally generous — a restic backup has no fixed
// absolute duration cap (see agent/internal/dispatcher/dispatcher.go), so a
// long-running-but-healthy backup must not be flagged as stalled.
const backupStallCheckInterval = 5 * time.Minute

// NewBackupStallJob marks Restic backup runs stuck in "running" for more than
// staleAfterMinutes as failed and notifies — a safety net for the case where
// the agent dies or loses connectivity before reporting run_backup's terminal
// status, which would otherwise leave the row "running" forever.
func NewBackupStallJob(svc *backupsvc.Service, staleAfterMinutes int) Job {
	return Job{
		Name: "backup-stall-check",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(backupStallCheckInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					svc.CheckStalledRuns(ctx, staleAfterMinutes)
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
