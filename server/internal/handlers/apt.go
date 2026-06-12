package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	aptsvc "github.com/serversupervisor/server/internal/services/apt"
)

// AptHandler translates HTTP to the apt service. Per-host access control
// (requireHostAccess) stays here as it needs the gin context; db is held only for
// that check. The dispatch + read logic lives in internal/services/apt.
type AptHandler struct {
	svc *aptsvc.Service
	db  *database.DB
}

func NewAptHandler(svc *aptsvc.Service, db *database.DB) *AptHandler {
	return &AptHandler{svc: svc, db: db}
}

func aptActor(c *gin.Context) string {
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}
	return username
}

// SendCommand sends an apt command to one or more hosts.
func (h *AptHandler) SendCommand(c *gin.Context) {
	var req models.AptCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	username := aptActor(c)

	var results []gin.H
	for _, hostID := range req.HostIDs {
		if !requireHostAccess(c, h.db, hostID, "operator") {
			results = append(results, gin.H{"host_id": hostID, "error": "host access denied"})
			continue
		}
		id, err := h.svc.Command(c.Request.Context(), hostID, req.Command, username, c.ClientIP())
		if err != nil {
			results = append(results, gin.H{"host_id": hostID, "error": err.Error()})
			continue
		}
		results = append(results, gin.H{"host_id": hostID, "command_id": id, "status": "pending"})
	}
	c.JSON(http.StatusOK, gin.H{"commands": results})
}

// GetCVESummary returns aggregated CVE severity counts across all hosts.
func (h *AptHandler) GetCVESummary(c *gin.Context) {
	summary, err := h.svc.CVESummary(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetAptStatus returns APT status for a host.
func (h *AptHandler) GetAptStatus(c *gin.Context) {
	status, err := h.svc.Status(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// GetUUStatus returns the unattended-upgrades status for a host.
func (h *AptHandler) GetUUStatus(c *gin.Context) {
	s, err := h.svc.UUStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// GetUURuns returns the upgrade run history for a host.
func (h *AptHandler) GetUURuns(c *gin.Context) {
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := h.svc.UURuns(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, runs)
}

// ConfigureUU applies a configuration update and/or enable/disable to unattended-upgrades.
func (h *AptHandler) ConfigureUU(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}
	var req models.UnattendedUpgradesConfigureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	commandIDs, err := h.svc.ConfigureUU(c.Request.Context(), hostID, req, aptActor(c), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_ids": commandIDs, "status": "pending"})
}

// InstallUU dispatches an apt-get install unattended-upgrades command to the agent.
func (h *AptHandler) InstallUU(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}
	id, err := h.svc.InstallUU(c.Request.Context(), hostID, aptActor(c), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": id, "status": "pending"})
}

// RunUUNow dispatches a manual unattended-upgrade run to the agent.
func (h *AptHandler) RunUUNow(c *gin.Context) {
	hostID := c.Param("id")
	if !requireHostAccess(c, h.db, hostID, "operator") {
		return
	}
	id, err := h.svc.RunUUNow(c.Request.Context(), hostID, aptActor(c), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": id, "status": "pending"})
}
