package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	hostsvc "github.com/serversupervisor/server/internal/services/host"
)

// HostHandler translates HTTP to the host service. Role authorization stays here
// (HTTP); all host logic lives in internal/services/host.
type HostHandler struct {
	svc *hostsvc.Service
}

func NewHostHandler(svc *hostsvc.Service) *HostHandler {
	return &HostHandler{svc: svc}
}

// RegisterHost creates a new host and returns its API key (admin only).
func (h *HostHandler) RegisterHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	var req models.HostRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	id, apiKey, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":      id,
		"api_key": apiKey,
		"message": "Host registered. Use this API key in the agent configuration. It will not be shown again.",
	})
}

// ListHosts returns all hosts.
func (h *HostHandler) ListHosts(c *gin.Context) {
	hosts, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, hosts)
}

// GetHost returns a specific host.
func (h *HostHandler) GetHost(c *gin.Context) {
	host, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, host)
}

// UpdateHost updates editable host fields.
func (h *HostHandler) UpdateHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	var req models.HostUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// TriggerAgentUpdate queues an agent self-update command for the host.
func (h *HostHandler) TriggerAgentUpdate(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	username := c.GetString("username")
	if username == "" {
		username = "system"
	}
	commandID, version, err := h.svc.TriggerAgentUpdate(c.Request.Context(), c.Param("id"), username, c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": commandID, "status": "pending", "target_version": version})
}

// DeleteHost removes a host.
func (h *HostHandler) DeleteHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "host deleted"})
}

// GetHostComplete returns a comprehensive snapshot used for initial page load.
func (h *HostHandler) GetHostComplete(c *gin.Context) {
	complete, err := h.svc.Complete(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, complete)
}

// RotateAPIKey regenerates an API key for a host (admin only).
func (h *HostHandler) RotateAPIKey(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	apiKey, err := h.svc.RotateKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"api_key": apiKey,
		"message": "API key rotated. Update the agent configuration immediately; it will not be shown again.",
	})
}

// GetHostDashboard returns complete host info (metrics + docker + apt).
func (h *HostHandler) GetHostDashboard(c *gin.Context) {
	dashboard, err := h.svc.Dashboard(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dashboard)
}
