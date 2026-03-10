package handlers

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/ws"
)

// validServiceName matches valid systemd service names: alphanumeric plus ._:@-
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9._:@\-]{1,256}$`)

type SystemHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	streamHub  *ws.CommandStreamHub
}

func NewSystemHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, streamHub *ws.CommandStreamHub) *SystemHandler {
	return &SystemHandler{db: db, cfg: cfg, dispatcher: dispatcher, streamHub: streamHub}
}

// SendJournalCommand enqueues a journalctl log fetch for a specific service on a host.
// Restricted to admin and operator roles (logs can contain sensitive data).
func (h *SystemHandler) SendJournalCommand(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req struct {
		HostID      string `json:"host_id" binding:"required"`
		ServiceName string `json:"service_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validServiceName.MatchString(req.ServiceName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name"})
		return
	}

	result, err := h.dispatcher.Create(dispatch.Request{
		HostID:      req.HostID,
		Module:      "journal",
		Action:      "logs",
		Target:      req.ServiceName,
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "journalctl",
			HostID:    req.HostID,
			IPAddress: c.ClientIP(),
			Details:   fmt.Sprintf(`{"service":"%s"}`, req.ServiceName),
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": result.Command.ID, "status": "pending"})
}

// SendProcessesCommand enqueues a process list snapshot request for an agent.
func (h *SystemHandler) SendProcessesCommand(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req struct {
		HostID string `json:"host_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.dispatcher.Create(dispatch.Request{
		HostID:      req.HostID,
		Module:      "processes",
		Action:      "list",
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "processes_list",
			HostID:    req.HostID,
			IPAddress: c.ClientIP(),
			Details:   "{}",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": result.Command.ID, "status": "pending"})
}

// SendSystemdCommand enqueues a systemd service management command for an agent.
func (h *SystemHandler) SendSystemdCommand(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req struct {
		HostID      string `json:"host_id" binding:"required"`
		ServiceName string `json:"service_name"`
		Action      string `json:"action" binding:"required,oneof=list start stop restart enable disable status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Action != "list" && !validServiceName.MatchString(req.ServiceName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service name"})
		return
	}

	result, err := h.dispatcher.Create(dispatch.Request{
		HostID:      req.HostID,
		Module:      "systemd",
		Action:      req.Action,
		Target:      req.ServiceName,
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "systemd_" + req.Action,
			HostID:    req.HostID,
			IPAddress: c.ClientIP(),
			Details:   fmt.Sprintf(`{"service":"%s","action":"%s"}`, req.ServiceName, req.Action),
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": result.Command.ID, "status": "pending"})
}
