package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// GetAgentAlertRuleCapabilities returns the agent (per-host) metric catalog.
func (h *AlertRulesHandler) GetAgentAlertRuleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": h.svc.AgentCapabilities()})
}

// GetSyntheticAlertRuleCapabilities returns the synthetic-monitoring metric catalog.
func (h *AlertRulesHandler) GetSyntheticAlertRuleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": h.svc.SyntheticCapabilities()})
}

// GetProxmoxAlertRuleCapabilities returns the Proxmox metric catalog + scope options.
func (h *AlertRulesHandler) GetProxmoxAlertRuleCapabilities(c *gin.Context) {
	resp, err := h.svc.ProxmoxCapabilities(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetDockerAlertRuleCapabilities returns the Docker metric catalog + per-host scope options.
func (h *AlertRulesHandler) GetDockerAlertRuleCapabilities(c *gin.Context) {
	resp, err := h.svc.DockerCapabilities(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetHostAlertMetrics returns alert metrics available for a host given its collectors.
func (h *AlertRulesHandler) GetHostAlertMetrics(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		respondError(c, apperr.Validation("hostId parameter is required"))
		return
	}
	resp, err := h.svc.HostMetrics(c.Request.Context(), hostID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
