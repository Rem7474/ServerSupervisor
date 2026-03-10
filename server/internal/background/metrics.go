package background

import (
	"context"
	"log"
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

					if n, err := db.BatchAggregateMetrics(start5, end, "5min"); err != nil {
						log.Printf("5min downsampling error: %v", err)
					} else if n > 0 {
						log.Printf("Downsampled 5min metrics for %d hosts", n)
					}

					if end.Minute() == 0 {
						if _, err := db.BatchAggregateMetrics(end.Add(-time.Hour), end, "hour"); err != nil {
							log.Printf("Hourly downsampling error: %v", err)
						}
					}

					if end.Hour() == 0 && end.Minute() == 0 {
						if _, err := db.BatchAggregateMetrics(end.Add(-24*time.Hour), end, "day"); err != nil {
							log.Printf("Daily downsampling error: %v", err)
						}
					}

				case <-ctx.Done():
					return
				}
			}
		},
	}
}
