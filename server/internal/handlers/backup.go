package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	backupsvc "github.com/serversupervisor/server/internal/services/backup"
)

// BackupHandler translates HTTP to the backup service. Per-host access control
// (requireHostAccess) stays here as it needs the gin context; db is held only
// for that check. Dispatch + history logic lives in internal/services/backup.
type BackupHandler struct {
	svc *backupsvc.Service
	db  *database.DB
}

func NewBackupHandler(svc *backupsvc.Service, db *database.DB) *BackupHandler {
	return &BackupHandler{svc: svc, db: db}
}

// SetBackgroundContext threads a long-lived ctx into the service for
// fire-and-forget completion notifications. Called once from main.go.
func (h *BackupHandler) SetBackgroundContext(ctx context.Context) {
	h.svc.SetBackgroundContext(ctx)
}

// HandleCommandCompletion implements agentsvc.CommandCompletionListener: it
// updates backup history when a restic run_backup command reaches a terminal
// state (no-ops for every other command).
func (h *BackupHandler) HandleCommandCompletion(commandID, status string) {
	h.svc.HandleCommandCompletion(commandID, status)
}

// GetStatus returns the aggregated backup status (latest run + passive state)
// for a host.
func (h *BackupHandler) GetStatus(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// ListRuns returns a host's backup run history.
func (h *BackupHandler) ListRuns(c *gin.Context) {
	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	runs, err := h.svc.ListRuns(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

// GetRun returns a single backup run by id.
func (h *BackupHandler) GetRun(c *gin.Context) {
	run, err := h.svc.GetRun(c.Request.Context(), c.Param("runId"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

// RunBackup dispatches a manual backup for a host.
func (h *BackupHandler) RunBackup(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}

	var req struct {
		Profile string `json:"profile"`
	}
	_ = c.ShouldBindJSON(&req) // body is optional — an empty POST runs the default profile

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	run, err := h.svc.TriggerBackup(c.Request.Context(), hostID, req.Profile, username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}
