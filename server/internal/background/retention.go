package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// NewMetricsRetentionJob trims the release_tracker_tag_digests table to the 100
// most recent entries per tracker, once per day. Raw metric retention is now
// owned by TimescaleDB retention policies (see migration 064 / the V2 baseline),
// so this job no longer deletes metric rows.
func NewMetricsRetentionJob(db *database.DB, cfg *config.Config) Job {
	_ = cfg // retained for signature compatibility; metric retention is Timescale-managed
	return Job{
		Name: "metrics-retention",
		Run: func(ctx context.Context) {
			// Run once at startup after a short delay, then every 24 hours.
			timer := time.NewTimer(5 * time.Minute)
			defer timer.Stop()
			for {
				select {
				case <-timer.C:
					if deleted, err := db.CleanupTrackerTagDigests(ctx, 100); err != nil {
						slog.ErrorContext(ctx, "tracker tag digests cleanup failed", slog.String("job", "metrics-retention"), slog.Any("err", err))
					} else if deleted > 0 {
						slog.InfoContext(ctx, "trimmed old tracker tag digests", slog.String("job", "metrics-retention"), slog.Int64("deleted", deleted))
					}

					// Reset to 24-hour interval after first run.
					timer.Reset(24 * time.Hour)

				case <-ctx.Done():
					return
				}
			}
		},
	}
}
