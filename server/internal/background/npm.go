package background

import (
	"context"
	"log/slog"
	"time"

	npmsvc "github.com/serversupervisor/server/internal/services/npm"
)

// NewNPMSyncJob periodically refreshes already-imported NPM proxy hosts
// (last_seen_at + npm_enabled). Per-connection poll_interval_sec is enforced
// inside RefreshAllEnabled; this ticker just wakes the service every 30 seconds
// to check which connections are due.
func NewNPMSyncJob(svc *npmsvc.Service) Job {
	return Job{
		Name: "npm-sync",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					slog.DebugContext(ctx, "npm-sync: refreshing enabled connections")
					svc.RefreshAllEnabled(ctx)
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
