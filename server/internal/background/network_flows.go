package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// NewNetworkFlowsRetentionJob purges old network_flow_metrics rows. Driven by
// an applicative job on a short default (not a fixed TimescaleDB retention
// policy) because remote_ip is a potentially identifying value — mirrors
// NewWebLogsRetentionJob's shape exactly.
func NewNetworkFlowsRetentionJob(db *database.DB, cfg *config.Config) Job {
	return Job{
		Name: "network-flows-retention",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					days := cfg.NetworkFlowsRetentionDays
					if days <= 0 {
						days = 14
					}
					if deleted, err := db.CleanOldNetworkFlowMetrics(ctx, days); err != nil {
						slog.ErrorContext(ctx, "network flows retention failed", slog.String("job", "network-flows-retention"), slog.Any("err", err))
					} else if deleted > 0 {
						slog.InfoContext(ctx, "deleted old network flow metrics", slog.String("job", "network-flows-retention"), slog.Int64("deleted", deleted), slog.Int("retention_days", days))
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
