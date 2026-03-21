package background

import (
	"context"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// NewAuditCleanupJob purges audit log entries older than cfg.AuditRetentionDays, once per hour.
func NewAuditCleanupJob(db *database.DB, cfg *config.Config) Job {
	return Job{
		Name: "audit-cleanup",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					days := cfg.AuditRetentionDays
					if days <= 0 {
						days = 90
					}
					if deleted, err := db.CleanOldAuditLogs(days); err != nil {
						log.Printf("Audit cleanup error: %v", err)
					} else if deleted > 0 {
						log.Printf("Cleaned up %d old audit log records (retention: %d days)", deleted, days)
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
