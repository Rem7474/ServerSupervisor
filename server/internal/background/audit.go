package background

import (
	"context"
	"log/slog"
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
					if deleted, err := db.CleanOldAuditLogs(ctx, days); err != nil {
						slog.ErrorContext(ctx, "audit cleanup failed", slog.String("job", "audit-cleanup"), slog.Any("err", err))
					} else if deleted > 0 {
						slog.InfoContext(ctx, "audit cleanup done", slog.String("job", "audit-cleanup"), slog.Int64("deleted", deleted), slog.Int("retention_days", days))
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
