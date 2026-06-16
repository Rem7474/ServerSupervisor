package background

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
)

// NewHostStatusJob marks hosts offline when no heartbeat has been received
// for more than 2 minutes. Runs every 30 seconds. When one or more hosts flip
// offline it wakes the live snapshots that render host status (bus is nil-safe).
func NewHostStatusJob(db *database.DB, bus *events.Bus) Job {
	return Job{
		Name: "host-status",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					changed, err := db.UpdateHostStatusBasedOnLastSeen(ctx, 2)
					if err != nil {
						slog.ErrorContext(ctx, "host status update failed", slog.String("job", "host-status"), slog.Any("err", err))
						continue
					}
					if changed > 0 {
						bus.PublishAll(events.TopicDashboard, events.TopicNetwork, events.TopicApt)
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
