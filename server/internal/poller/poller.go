// Package poller runs a function on a fixed interval until its context is
// cancelled. It decouples background *scheduling* from the HTTP handlers that own
// the actual work: handlers expose a poll-once operation, main wires the schedule.
package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/safego"
)

// Every runs tick on the given interval. When immediate is true it also fires tick
// once right away. Both the immediate pass and the ticked passes stop when ctx is
// cancelled (typically the SIGTERM-bound root context), so no explicit Stop is
// needed. name labels the startup log line.
//
// Each tick is individually panic-recovered so one bad tick logs and skips
// instead of silently killing the poller (or, unrecovered, the whole process)
// for good.
func Every(ctx context.Context, interval time.Duration, immediate bool, name string, tick func(context.Context)) {
	runTick := func() {
		defer safego.Recover(ctx, name)
		tick(ctx)
	}
	if immediate {
		go runTick()
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				runTick()
			case <-ctx.Done():
				return
			}
		}
	}()
	slog.InfoContext(ctx, "poller started", slog.String("name", name), slog.Duration("interval", interval))
}
