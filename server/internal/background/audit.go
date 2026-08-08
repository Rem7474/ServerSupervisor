package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// NewAuditCleanupJob purges audit log entries per category, once per hour —
// a category present in cfg.AuditRetentionDaysByCategory uses its own
// threshold, everything else falls back to cfg.AuditRetentionDays (see
// ROADMAP.md item #13). Iterating models.AuditCategories() rather than a
// single global DELETE means a "keep alerts for a year, commands for a
// month" split retention policy is one settings change, not a database
// change.
func NewAuditCleanupJob(db *database.DB, cfg *config.Config) Job {
	return Job{
		Name: "audit-cleanup",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					defaultDays := cfg.AuditRetentionDays
					if defaultDays <= 0 {
						defaultDays = 90
					}
					for _, cat := range models.AuditCategories() {
						days := defaultDays
						if override, ok := cfg.AuditRetentionDaysByCategory[cat.Key]; ok && override > 0 {
							days = override
						}
						deleted, err := db.CleanOldAuditLogsByCategory(ctx, cat.Key, days)
						if err != nil {
							slog.ErrorContext(ctx, "audit cleanup failed", slog.String("job", "audit-cleanup"), slog.String("category", cat.Key), slog.Any("err", err))
						} else if deleted > 0 {
							slog.InfoContext(ctx, "audit cleanup done", slog.String("job", "audit-cleanup"), slog.String("category", cat.Key), slog.Int64("deleted", deleted), slog.Int("retention_days", days))
						}
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
