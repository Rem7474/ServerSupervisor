package background

import (
	"context"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/database"
)

// NewHostStatusJob marks hosts offline when no heartbeat has been received
// for more than 2 minutes. Runs every 30 seconds.
func NewHostStatusJob(db *database.DB) Job {
	return Job{
		Name: "host-status",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := db.UpdateHostStatusBasedOnLastSeen(2); err != nil {
						log.Printf("Failed to update host status: %v", err)
					}
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
