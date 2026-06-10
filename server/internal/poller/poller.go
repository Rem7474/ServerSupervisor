// Package poller runs a function on a fixed interval until its context is
// cancelled. It decouples background *scheduling* from the HTTP handlers that own
// the actual work: handlers expose a poll-once operation, main wires the schedule.
package poller

import (
	"context"
	"log/slog"
	"time"
)

// Every runs tick on the given interval. When immediate is true it also fires tick
// once right away. Both the immediate pass and the ticked passes stop when ctx is
// cancelled (typically the SIGTERM-bound root context), so no explicit Stop is
// needed. name labels the startup log line.
func Every(ctx context.Context, interval time.Duration, immediate bool, name string, tick func(context.Context)) {
	if immediate {
		go tick(ctx)
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				tick(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	slog.InfoContext(ctx, "poller started", slog.String("name", name), slog.Duration("interval", interval))
}
