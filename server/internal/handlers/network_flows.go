package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// GetNetworkFlows retourne les top talkers du dernier cycle de rapport pour un hôte.
func (h *HostHandler) GetNetworkFlows(c *gin.Context) {
	flows, err := h.svc.NetworkFlows(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, flows)
}

// GetNetworkFlowsHistory retourne l'historique de bande passante d'un talker (IP/port/protocole).
func (h *HostHandler) GetNetworkFlowsHistory(c *gin.Context) {
	remoteIP := c.Query("remote_ip")
	protocol := c.Query("protocol")
	if remoteIP == "" || protocol == "" {
		respondError(c, apperr.Validation("remote_ip and protocol query parameters required"))
		return
	}
	remotePort, _ := strconv.Atoi(c.Query("remote_port"))
	hours, err := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if err != nil || hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	points, err := h.svc.NetworkFlowsHistory(c.Request.Context(), c.Param("id"), remoteIP, remotePort, protocol, hours)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"hours":       hours,
		"remote_ip":   remoteIP,
		"remote_port": remotePort,
		"protocol":    protocol,
		"points":      points,
	})
}

// GetNetworkFlowsSummary retourne la bande passante totale trackée d'un hôte dans le temps.
func (h *HostHandler) GetNetworkFlowsSummary(c *gin.Context) {
	hours, err := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if err != nil || hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	points, err := h.svc.NetworkFlowsSummary(c.Request.Context(), c.Param("id"), hours)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"hours":  hours,
		"points": points,
	})
}
