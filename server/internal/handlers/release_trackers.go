package handlers

import (
	"context"
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
	pollerCtx  context.Context // detached ctx for fire-and-forget goroutines (check-now, NotifyComplete)
}

func NewReleaseTrackerHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *ReleaseTrackerHandler {
	return &ReleaseTrackerHandler{
		db:         db,
		cfg:        cfg,
		dispatcher: dispatcher,
		notifHub:   notifHub,
		pollerCtx:  context.Background(), // placeholder; real ctx set via SetBackgroundContext
	}
}

// PollInterval returns the configured poll cadence (default 15m).
func (h *ReleaseTrackerHandler) PollInterval() time.Duration {
	if h.cfg.GitHubPollInterval == 0 {
		return 15 * time.Minute
	}
	return h.cfg.GitHubPollInterval
}

// SetBackgroundContext threads a long-lived (SIGTERM-bound) ctx into the handler
// for the fire-and-forget goroutines spawned from HTTP requests (check-now,
// NotifyComplete). Called once from main.go; the periodic loop is owned by the
// poller package.
func (h *ReleaseTrackerHandler) SetBackgroundContext(ctx context.Context) {
	h.pollerCtx = ctx
}

// CheckAll polls every enabled release tracker once. Scheduling is owned by the
// poller package (poller.Every); this is the unit of work it ticks.
func (h *ReleaseTrackerHandler) CheckAll(ctx context.Context) {
	h.checkAll(ctx)
}
