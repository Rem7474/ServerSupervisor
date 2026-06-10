package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	auditsvc "github.com/serversupervisor/server/internal/services/audit"
)

// AuditHandler translates HTTP to the audit service. Role authorization, query
// parsing / limit clamping and response envelopes stay here (HTTP concerns); the
// read orchestration lives in internal/services/audit.
type AuditHandler struct {
	svc *auditsvc.Service
}

func NewAuditHandler(svc *auditsvc.Service) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// clampQueryInt reads a positive query int, applying a default and a max cap.
func clampQueryInt(c *gin.Context, key string, def, max int) int {
	v := def
	if raw := c.Query(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= max {
			v = parsed
		}
	}
	return v
}

// GetAuditLogs returns paginated audit logs (admin only).
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	page := clampQueryInt(c, "page", 1, 1<<31-1)
	limit := clampQueryInt(c, "limit", 50, 100)
	logs, err := h.svc.Logs(c.Request.Context(), limit, (page-1)*limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs, "page": page, "limit": limit})
}

// GetAuditLogsByHost returns audit logs for a specific host.
func (h *AuditHandler) GetAuditLogsByHost(c *gin.Context) {
	hostID := c.Param("host_id")
	if hostID == "" {
		respondError(c, apperr.Validation("host_id required"))
		return
	}
	logs, err := h.svc.LogsByHost(c.Request.Context(), hostID, clampQueryInt(c, "limit", 100, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, logs)
}

// GetMyAuditLogs returns the current user's own audit logs.
func (h *AuditHandler) GetMyAuditLogs(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	logs, err := h.svc.LogsByUser(c.Request.Context(), username, clampQueryInt(c, "limit", 10, 100))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": username, "logs": logs})
}

// GetCommandsHistory returns paginated remote commands for all hosts (admin and operator).
func (h *AuditHandler) GetCommandsHistory(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	page := clampQueryInt(c, "page", 1, 1<<31-1)
	limit := 50
	f := database.CommandFilter{
		Search: c.Query("search"),
		Module: c.Query("module"),
		Status: c.Query("status"),
	}
	cmds, total, err := h.svc.Commands(c.Request.Context(), limit, (page-1)*limit, f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": cmds, "total": total, "page": page, "limit": limit})
}

// GetCommandByID returns the status and output of a single remote command by UUID.
// Requires admin or operator role to prevent cross-host information disclosure.
func (h *AuditHandler) GetCommandByID(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	id := c.Param("id")
	if id == "" {
		respondError(c, apperr.Validation("id required"))
		return
	}
	cmd, err := h.svc.Command(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cmd)
}

// GetAuditLogsByUser returns audit logs for a specific user (admin only).
func (h *AuditHandler) GetAuditLogsByUser(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	username := c.Param("username")
	if username == "" {
		respondError(c, apperr.Validation("username required"))
		return
	}
	logs, err := h.svc.LogsByUser(c.Request.Context(), username, clampQueryInt(c, "limit", 100, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": username, "logs": logs})
}
