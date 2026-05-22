package background

import (
	"context"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/synthetic"
)

// NewUptimeWorkerJob runs the synthetic uptime probe loop.
// Each probe is checked at its own interval; the worker wakes every 10 seconds.
func NewUptimeWorkerJob(db *database.DB) Job {
	return Job{
		Name: "uptime-worker",
		Run: func(ctx context.Context) {
			synthetic.RunUptimeWorker(ctx, db)
		},
	}
}

// NewSSLWorkerJob runs the SSL/TLS certificate expiration checker.
// Checks all enabled certificates every 6 hours.
func NewSSLWorkerJob(db *database.DB) Job {
	return Job{
		Name: "ssl-worker",
		Run: func(ctx context.Context) {
			synthetic.RunSSLWorker(ctx, db)
		},
	}
}
