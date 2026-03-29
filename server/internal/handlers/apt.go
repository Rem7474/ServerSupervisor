package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type AptHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
}

func NewAptHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher) *AptHandler {
	return &AptHandler{db: db, cfg: cfg, dispatcher: dispatcher}
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
		if !requireHostAccess(c, h.db, hostID, "operator") {
			results = append(results, gin.H{"host_id": hostID, "error": "host access denied"})
			continue
		}

		result, err := h.dispatcher.Create(dispatch.Request{
			HostID:      hostID,
			Module:      "apt",
			Action:      req.Command,
			Payload:     "{}",
			TriggeredBy: username,
			Audit: &dispatch.AuditLogRequest{
				Username:  username,
				Action:    "apt_" + req.Command,
				HostID:    hostID,
				IPAddress: c.ClientIP(),
				Details:   "apt " + req.Command,
			},
		})
		if err != nil {
			results = append(results, gin.H{"host_id": hostID, "error": err.Error()})
			continue
		}

		results = append(results, gin.H{"host_id": hostID, "command_id": result.Command.ID, "status": "pending"})
	}

	c.JSON(http.StatusOK, gin.H{"commands": results})
}

// GetCVESummary returns aggregated CVE severity counts across all hosts.
func (h *AptHandler) GetCVESummary(c *gin.Context) {
	summary, err := h.db.GetAptCVESummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
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
