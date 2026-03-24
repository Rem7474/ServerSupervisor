package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/errors"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// ListNodes returns all nodes, optionally filtered by connection_id query param.
func (h *ProxmoxHandler) ListNodes(c *gin.Context) {
	connID := c.Query("connection_id")
	var (
		nodes []models.ProxmoxNode
		err   error
	)
	if connID != "" {
		nodes, err = h.db.ListProxmoxNodesByConnection(connID)
	} else {
		nodes, err = h.db.ListProxmoxNodes()
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// GetNode returns a single node with its guests and storages.
func (h *ProxmoxHandler) GetNode(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, lang))
		return
	}
	c.JSON(http.StatusOK, node)
}

// GetNodeMetricsSummary returns time-bucketed avg CPU% and RAM% across all Proxmox nodes.
// Same query parameters as GET /metrics/summary for agent hosts.
func (h *ProxmoxHandler) GetNodeMetricsSummary(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	bucketMinutes, _ := strconv.Atoi(c.DefaultQuery("bucket_minutes", "5"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	summary, err := h.db.GetProxmoxNodeMetricsSummary(hours, bucketMinutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if summary == nil {
		summary = []models.ProxmoxNodeMetricsSummary{}
	}
	c.JSON(http.StatusOK, summary)
}

// ─── Disks ────────────────────────────────────────────────────────────────────

// ListNodeDisks returns physical disks for a node identified by its UUID.
func (h *ProxmoxHandler) ListNodeDisks(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	disks, err := h.db.ListProxmoxDisksByNode(node.ConnectionID, node.NodeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, disks)
}

// ─── Services ─────────────────────────────────────────────────────────────────

// ListNodeServices returns all systemd services on a Proxmox node. Requires Sys.Audit.
func (h *ProxmoxHandler) ListNodeServices(c *gin.Context) {
	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))))
		return
	}

	secret, conn, err := h.resolveSecret(node.ConnectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	services, err := client.GetNodeServices(node.NodeName)
	if err != nil {
		log.Printf("proxmox services list [%s/%s]: %v", conn.Name, node.NodeName, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}
