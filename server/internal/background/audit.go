package background

import (
	"context"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/database"
)

// NewAuditCleanupJob purges audit log entries older than 90 days, once per hour.
func NewAuditCleanupJob(db *database.DB) Job {
	return Job{
		Name: "audit-cleanup",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if deleted, err := db.CleanOldAuditLogs(90); err != nil {
						log.Printf("Audit cleanup error: %v", err)
					} else if deleted > 0 {
						log.Printf("Cleaned up %d old audit log records", deleted)
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
