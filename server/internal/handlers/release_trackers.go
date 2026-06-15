package handlers

import (
	"context"
	"time"

	releasetrackersvc "github.com/serversupervisor/server/internal/services/releasetracker"
)

// ReleaseTrackerHandler translates HTTP to the release-tracker service and owns
// the detached ctx for fire-and-forget goroutines (check-now, NotifyComplete).
// CRUD, polling, dispatch and notifications live in internal/services/releasetracker.
type ReleaseTrackerHandler struct {
	svc       *releasetrackersvc.Service
	pollerCtx context.Context
}

func NewReleaseTrackerHandler(svc *releasetrackersvc.Service) *ReleaseTrackerHandler {
	return &ReleaseTrackerHandler{svc: svc, pollerCtx: context.Background()}
}

// PollInterval returns the configured poll cadence (default 15m).
func (h *ReleaseTrackerHandler) PollInterval() time.Duration {
	return h.svc.PollInterval()
}

// SetBackgroundContext threads a long-lived (SIGTERM-bound) ctx for the
// fire-and-forget goroutines spawned from HTTP requests (check-now, NotifyComplete).
func (h *ReleaseTrackerHandler) SetBackgroundContext(ctx context.Context) {
	h.pollerCtx = ctx
}

// CheckAll polls every enabled tracker once (scheduling owned by poller.Every).
func (h *ReleaseTrackerHandler) CheckAll(ctx context.Context) {
	h.svc.CheckAll(ctx)
}

// HandleCommandCompletion implements CommandCompletionListener: it notifies the
// service when a tracker-triggered command reaches a terminal state.
func (h *ReleaseTrackerHandler) HandleCommandCompletion(commandID, status string) {
	h.svc.NotifyComplete(h.pollerCtx, commandID, status)
}
