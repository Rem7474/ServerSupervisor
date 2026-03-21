package background

import (
	"context"
	"log"
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
					if deleted, err := db.CleanOldMetrics(days); err != nil {
						log.Printf("Metrics retention error: %v", err)
					} else if deleted > 0 {
						log.Printf("Deleted %d old metric rows (retention: %d days)", deleted, days)
					}

					if deleted, err := db.CleanupTrackerTagDigests(100); err != nil {
						log.Printf("Tracker tag digests cleanup error: %v", err)
					} else if deleted > 0 {
						log.Printf("Trimmed %d old tracker tag digest rows", deleted)
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
