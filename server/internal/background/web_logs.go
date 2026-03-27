package background

import (
	"context"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

func NewWebLogsRetentionJob(db *database.DB, cfg *config.Config) Job {
	return Job{
		Name: "web-logs-retention",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					days := cfg.WebLogsRetentionDays
					if days <= 0 {
						days = 30
					}
					if deleted, err := db.CleanOldWebLogs(days); err != nil {
						log.Printf("Web logs retention error: %v", err)
					} else if deleted > 0 {
						log.Printf("Deleted %d old web log snapshots (retention: %d days)", deleted, days)
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
