package api

import (
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// validServiceName matches valid systemd service names: alphanumeric plus ._:@-
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9._:@\-]{1,256}$`)

type SystemHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *AptStreamHub
}

func NewSystemHandler(db *database.DB, cfg *config.Config, streamHub *AptStreamHub) *SystemHandler {
	return &SystemHandler{db: db, cfg: cfg, streamHub: streamHub}
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

	details := fmt.Sprintf(`{"service":"%s"}`, req.ServiceName)
	auditID, auditErr := h.db.CreateAuditLog(username, "journalctl", req.HostID, c.ClientIP(), details, "pending")
	var auditLogIDPtr *int64
	if auditErr != nil {
		log.Printf("Warning: failed to create audit log for journalctl command: %v", auditErr)
	} else {
		auditLogIDPtr = &auditID
	}

	cmd, err := h.db.CreateRemoteCommand(req.HostID, "journal", "logs", req.ServiceName, "{}", username, auditLogIDPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": cmd.ID, "status": "pending"})
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

	details := fmt.Sprintf(`{"service":"%s","action":"%s"}`, req.ServiceName, req.Action)
	auditID, auditErr := h.db.CreateAuditLog(username, "systemd_"+req.Action, req.HostID, c.ClientIP(), details, "pending")
	var auditLogIDPtr *int64
	if auditErr != nil {
		log.Printf("Warning: failed to create audit log for systemd command: %v", auditErr)
	} else {
		auditLogIDPtr = &auditID
	}

	cmd, err := h.db.CreateRemoteCommand(req.HostID, "systemd", req.Action, req.ServiceName, "{}", username, auditLogIDPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": cmd.ID, "status": "pending"})
}
