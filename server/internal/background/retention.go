package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// NewMetricsRetentionJob deletes raw metrics older than cfg.MetricsRetentionDays once per day,
// and trims the release_tracker_tag_digests table to the 100 most recent entries per tracker.
func NewMetricsRetentionJob(db *database.DB, cfg *config.Config) Job {
	return Job{
		Name: "metrics-retention",
		Run: func(ctx context.Context) {
			// Run once at startup after a short delay, then every 24 hours.
			timer := time.NewTimer(5 * time.Minute)
			defer timer.Stop()
			for {
				select {
				case <-timer.C:
					days := cfg.MetricsRetentionDays
					if days <= 0 {
						days = 30
					}
					if deleted, err := db.CleanOldMetrics(ctx, days); err != nil {
						slog.ErrorContext(ctx, "metrics retention failed", slog.String("job", "metrics-retention"), slog.Any("err", err))
					} else if deleted > 0 {
						slog.InfoContext(ctx, "deleted old metric rows", slog.String("job", "metrics-retention"), slog.Int64("deleted", deleted), slog.Int("retention_days", days))
					}

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
