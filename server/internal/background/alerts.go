package background

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// NewAlertEvalJob evaluates all alert rules against current host metrics every 60 seconds.
// pusher receives real-time browser push events on alert fire; pass nil to disable.
func NewAlertEvalJob(db *database.DB, cfg *config.Config, pusher alerts.NotificationPusher) Job {
	return Job{
		Name: "alert-eval",
		Run: func(ctx context.Context) {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					alerts.EvaluateAlerts(db, cfg, pusher)
				case <-ctx.Done():
					return
				}
			}
		},
	}
}
