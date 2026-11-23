package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	systemsvc "github.com/serversupervisor/server/internal/services/system"
)

// SystemHandler exposes host-level system actions. Request binding and
// authorization are handled here; the command construction lives in
// internal/services/system.
type SystemHandler struct {
	access hostAccessReader
	svc    *systemsvc.Service
}

func NewSystemHandler(access hostAccessReader, svc *systemsvc.Service) *SystemHandler {
	return &SystemHandler{access: access, svc: svc}
}

// requireOperator enforces the global admin/operator role: these actions can
// expose sensitive data (logs, process lists) or change services.
func requireOperator(c *gin.Context) bool {
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return false
	}
	return true
}

func (h *SystemHandler) actor(c *gin.Context) systemsvc.Actor {
	return systemsvc.Actor{Username: c.GetString("username"), IPAddress: c.ClientIP()}
}

func respondCommandQueued(c *gin.Context, commandID string) {
	c.JSON(http.StatusOK, gin.H{"command_id": commandID, "status": "pending"})
}

// SendJournalCommand enqueues a journalctl log fetch for a specific service on a host.
// Restricted to admin and operator roles (logs can contain sensitive data).
func (h *SystemHandler) SendJournalCommand(c *gin.Context) {
	if !requireOperator(c) {
		return
	}
	var req struct {
		HostID      string `json:"host_id" binding:"required"`
		ServiceName string `json:"service_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if !requireHostAccess(c, h.access, req.HostID, "operator") {
		return
	}
	id, err := h.svc.JournalLogs(c.Request.Context(), h.actor(c), req.HostID, req.ServiceName)
	if err != nil {
		respondError(c, err)
		return
	}
	respondCommandQueued(c, id)
}

// SendProcessesCommand enqueues a process list snapshot request for an agent.
func (h *SystemHandler) SendProcessesCommand(c *gin.Context) {
	if !requireOperator(c) {
		return
	}
	var req struct {
		HostID string `json:"host_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if !requireHostAccess(c, h.access, req.HostID, "operator") {
		return
	}
	id, err := h.svc.ProcessesList(c.Request.Context(), h.actor(c), req.HostID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondCommandQueued(c, id)
}

// SendSystemdCommand enqueues a systemd service management command for an agent.
func (h *SystemHandler) SendSystemdCommand(c *gin.Context) {
	if !requireOperator(c) {
		return
	}
	var req struct {
		HostID      string `json:"host_id" binding:"required"`
		ServiceName string `json:"service_name"`
		Action      string `json:"action" binding:"required,oneof=list start stop restart enable disable status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if !requireHostAccess(c, h.access, req.HostID, "operator") {
		return
	}
	id, err := h.svc.Systemd(c.Request.Context(), h.actor(c), req.HostID, req.ServiceName, req.Action)
	if err != nil {
		respondError(c, err)
		return
	}
	respondCommandQueued(c, id)
}
