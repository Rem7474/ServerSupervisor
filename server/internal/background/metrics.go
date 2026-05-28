package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/database"
)

// NewMetricsDownsampleJob aggregates raw metrics into 5-minute, hourly, and daily
// buckets. Runs every 5 minutes; hourly and daily passes are triggered automatically
// when the wall clock crosses the relevant boundary.
func NewMetricsDownsampleJob(db *database.DB) Job {
	return Job{
		Name: "metrics-downsample",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					now := time.Now().UTC()
					end := now.Truncate(5 * time.Minute)
					start5 := end.Add(-5 * time.Minute)

					if n, err := db.BatchAggregateMetrics(ctx, start5, end, "5min"); err != nil {
						slog.ErrorContext(ctx, "5min downsampling failed", slog.String("job", "metrics-downsample"), slog.Any("err", err))
					} else if n > 0 {
						slog.InfoContext(ctx, "downsampled 5min metrics", slog.String("job", "metrics-downsample"), slog.Int("hosts", n))
					}

					if end.Minute() == 0 {
						if _, err := db.BatchAggregateMetrics(ctx, end.Add(-time.Hour), end, "hour"); err != nil {
							slog.ErrorContext(ctx, "hourly downsampling failed", slog.String("job", "metrics-downsample"), slog.Any("err", err))
						}
					}

					if end.Hour() == 0 && end.Minute() == 0 {
						if _, err := db.BatchAggregateMetrics(ctx, end.Add(-24*time.Hour), end, "day"); err != nil {
							slog.ErrorContext(ctx, "daily downsampling failed", slog.String("job", "metrics-downsample"), slog.Any("err", err))
						}
					}

				case <-ctx.Done():
					return
				}
			}
		},
	}
}
