package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/ws"
)

var validReleaseProviders = map[string]bool{
	"github": true, "gitlab": true, "gitea": true,
}

// validateDockerTracker normalizes and validates a docker tracker's deployment
// mode. Returns (httpStatus, message); status 0 means valid. Compose mode
// requires host + project; custom mode keeps the legacy monitor-only/task rules.
func validateDockerTracker(req *models.ReleaseTracker) (int, string) {
	if req.UpdateAction == "" {
		req.UpdateAction = "custom"
	}
	if req.UpdateAction != "custom" && req.UpdateAction != "compose" {
		return http.StatusBadRequest, "update_action must be 'custom' or 'compose'"
	}
	if req.DockerImage == "" {
		return http.StatusBadRequest, "docker_image is required for docker trackers"
	}
	if req.DockerTag == "" {
		req.DockerTag = "latest"
	}
	if req.UpdateAction == "compose" {
		if req.HostID == "" || req.ComposeProject == "" {
			return http.StatusBadRequest, "host_id and compose_project are required for compose update mode"
		}
	} else if req.HostID != "" && req.CustomTaskID == "" {
		return http.StatusBadRequest, "custom_task_id is required when host_id is set"
	}
	if req.HealthcheckTimeoutSec < 0 || req.HealthcheckTimeoutSec > 3600 {
		return http.StatusBadRequest, "healthcheck_timeout_sec must be between 0 and 3600"
	}
	return 0, ""
}

type ReleaseTrackerHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	notifHub   *ws.NotificationHub
	pollerCtx  context.Context // detached ctx for goroutines fired from handlers (check-now)
	cancel     context.CancelFunc
}

func NewReleaseTrackerHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *ReleaseTrackerHandler {
	return &ReleaseTrackerHandler{
		db:         db,
		cfg:        cfg,
		dispatcher: dispatcher,
		notifHub:   notifHub,
		pollerCtx:  context.Background(), // placeholder; real ctx is set in StartPoller
	}
}

// StartPoller begins periodic polling of release trackers.
// The provided parent ctx is propagated to every DB call; cancelling it (or
// calling StopPoller) terminates the loop.
func (h *ReleaseTrackerHandler) StartPoller(parent context.Context) {
	interval := h.cfg.GitHubPollInterval
	if interval == 0 {
		interval = 15 * time.Minute
	}
	log.Printf("Release tracker poller started (interval: %v)", interval)

	ctx, cancel := context.WithCancel(parent)
	h.pollerCtx = ctx
	h.cancel = cancel

	go h.checkAll(ctx)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.checkAll(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (h *ReleaseTrackerHandler) StopPoller() {
	if h.cancel != nil {
		h.cancel()
	}
}
