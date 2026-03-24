package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/errors"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// ─── Live node status (proxied from PVE, not stored in DB) ───────────────────

// GetNodeStatus proxies GET /nodes/{node}/status from PVE.
// Returns real-time iowait, swap, rootfs — not cached in DB.
func (h *ProxmoxHandler) GetNodeStatus(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	secret, conn, err := h.resolveSecret(node.ConnectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	status, err := client.GetNodeStatus(node.NodeName)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// ─── RRD metrics ──────────────────────────────────────────────────────────────

// GetNodeRRD proxies GET /nodes/{node}/rrddata from PVE.
// Accepts ?timeframe=hour|day|week|month|year (default: hour).
func (h *ProxmoxHandler) GetNodeRRD(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	timeframe := c.DefaultQuery("timeframe", "hour")
	switch timeframe {
	case "hour", "day", "week", "month", "year":
	default:
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(errors.CodeInvalidTimeframe, lang))
		return
	}

	secret, conn, err := h.resolveSecret(node.ConnectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	points, err := client.GetNodeRRDData(node.NodeName, timeframe)
	if err != nil {
		log.Printf("proxmox rrd [%s/%s] %s: %v", conn.Name, node.NodeName, timeframe, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, points)
}
