package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AptHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAptHandler(db *database.DB, cfg *config.Config) *AptHandler {
	return &AptHandler{db: db, cfg: cfg}
}

// SendCommand sends an apt command to one or more hosts
func (h *AptHandler) SendCommand(c *gin.Context) {
	var req models.AptCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	tpRole := c.GetString("role")

	// Only admin and operator can trigger APT commands
	if tpRole != models.RoleAdmin && tpRole != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var results []gin.H
	for _, hostID := range req.HostIDs {
		// Log to audit trail
		ip := c.ClientIP()
		auditDetails := "apt " + req.Command
		auditLogID, err := h.db.CreateAuditLog(username, "apt_"+req.Command, hostID, ip, auditDetails, "pending")
		if err != nil {
			// Log audit failure but don't block the request
			auditLogID = 0
		}

		var auditLogIDPtr *int64
		if auditLogID != 0 {
			auditLogIDPtr = &auditLogID
		}

		cmd, err := h.db.CreateAptCommand(hostID, req.Command, username, auditLogIDPtr)
		if err != nil {
			if auditLogIDPtr != nil {
				_ = h.db.UpdateAuditLogStatus(*auditLogIDPtr, "failed", err.Error())
			}
			results = append(results, gin.H{"host_id": hostID, "error": err.Error()})
			continue
		}

		results = append(results, gin.H{"host_id": hostID, "command_id": cmd.ID, "status": "pending"})
	}

	c.JSON(http.StatusOK, gin.H{"commands": results})
}

// GetAptStatus returns APT status for a host
func (h *AptHandler) GetAptStatus(c *gin.Context) {
	hostID := c.Param("id")
	status, err := h.db.GetAptStatus(hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "apt status not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// GetCommandHistory returns APT command history for a host
func (h *AptHandler) GetCommandHistory(c *gin.Context) {
	hostID := c.Param("id")
	cmds, err := h.db.GetAptCommandHistory(hostID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch command history"})
		return
	}
	if cmds == nil {
		cmds = []models.AptCommand{}
	}
	c.JSON(http.StatusOK, cmds)
}
