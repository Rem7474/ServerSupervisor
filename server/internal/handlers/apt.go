package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	summary, err := h.db.GetAptCVESummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetAptStatus returns APT status for a host
func (h *AptHandler) GetAptStatus(c *gin.Context) {
	hostID := c.Param("id")
	status, err := h.db.GetAptStatus(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "apt status not found"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// ========== Unattended-Upgrades handlers ==========

// GetUUStatus returns the unattended-upgrades status for a host.
func (h *AptHandler) GetUUStatus(c *gin.Context) {
	hostID := c.Param("id")
	s, err := h.db.GetUUStatus(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// GetUURuns returns the upgrade run history for a host.
func (h *AptHandler) GetUURuns(c *gin.Context) {
	hostID := c.Param("id")
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := h.db.GetUURuns(c.Request.Context(), hostID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []models.UURun{}
	}
	c.JSON(http.StatusOK, runs)
}

// ConfigureUU applies a configuration update and/or enable/disable to unattended-upgrades.
// Dispatches configure_uu and optionally toggle_uu commands to the agent.
func (h *AptHandler) ConfigureUU(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}

	var req models.UnattendedUpgradesConfigureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	cfgPayload, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode config"})
		return
	}

	var commandIDs []string

	// Dispatch configure_uu
	r, err := h.dispatcher.Create(dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "configure_uu",
		Payload:     string(cfgPayload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "uu_configure",
			HostID:    hostID,
			IPAddress: c.ClientIP(),
			Details:   "configure unattended-upgrades",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	commandIDs = append(commandIDs, r.Command.ID)

	// Dispatch toggle_uu to enable or disable the service
	target := "disable"
	if req.Enabled {
		target = "enable"
	}
	r2, err := h.dispatcher.Create(dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "toggle_uu",
		Target:      target,
		Payload:     "{}",
		TriggeredBy: username,
	})
	if err == nil {
		commandIDs = append(commandIDs, r2.Command.ID)
	}

	c.JSON(http.StatusOK, gin.H{"command_ids": commandIDs, "status": "pending"})
}

// InstallUU dispatches an apt-get install unattended-upgrades command to the agent.
func (h *AptHandler) InstallUU(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	r, err := h.dispatcher.Create(dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "install_uu",
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "uu_install",
			HostID:    hostID,
			IPAddress: c.ClientIP(),
			Details:   "install unattended-upgrades",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": r.Command.ID, "status": "pending"})
}

// RunUUNow dispatches a manual unattended-upgrade run to the agent.
func (h *AptHandler) RunUUNow(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	r, err := h.dispatcher.Create(dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "run_uu",
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "uu_run",
			HostID:    hostID,
			IPAddress: c.ClientIP(),
			Details:   "manual unattended-upgrade run",
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": r.Command.ID, "status": "pending"})
}
