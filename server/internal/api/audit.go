package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)


type AuditHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAuditHandler(db *database.DB, cfg *config.Config) *AuditHandler {
	return &AuditHandler{db: db, cfg: cfg}
}

// GetAuditLogs returns paginated audit logs (admin only)
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	// Check if user is admin
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	// Get pagination params
	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := (page - 1) * limit

	logs, err := h.db.GetAuditLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch audit logs"})
		return
	}

	if logs == nil {
		logs = []models.AuditLog{}
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"page":  page,
		"limit": limit,
	})
}

// GetAuditLogsByHost returns audit logs for a specific host
func (h *AuditHandler) GetAuditLogsByHost(c *gin.Context) {
	hostID := c.Param("host_id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id required"})
		return
	}

	// Get limit param
	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	logs, err := h.db.GetAuditLogsByHost(hostID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch audit logs"})
		return
	}

	if logs == nil {
		logs = []models.AuditLog{}
	}

	c.JSON(http.StatusOK, logs)
}

// GetMyAuditLogs returns the current user's own audit logs
func (h *AuditHandler) GetMyAuditLogs(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	logs, err := h.db.GetAuditLogsByUser(username, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": username,
		"logs": logs,
	})
}

// GetCommandsHistory returns paginated remote commands for all hosts (admin and operator).
func (h *AuditHandler) GetCommandsHistory(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	page, limit := 1, 50
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	offset := (page - 1) * limit

	cmds, err := h.db.GetAllRemoteCommands(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch commands history"})
		return
	}
	total, _ := h.db.CountAllRemoteCommands()
	if cmds == nil {
		cmds = []database.RemoteCommandWithHost{}
	}
	c.JSON(http.StatusOK, gin.H{"commands": cmds, "total": total, "page": page, "limit": limit})
}

// GetAuditLogsByUser returns audit logs for a specific user (admin only)
func (h *AuditHandler) GetAuditLogsByUser(c *gin.Context) {
	// Check if user is admin
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username required"})
		return
	}

	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	logs, err := h.db.GetAuditLogsByUser(username, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": username,
		"logs": logs,
	})
}
