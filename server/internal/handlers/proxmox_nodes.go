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

// ─── Tasks ────────────────────────────────────────────────────────────────────

// ListTasks returns recent tasks, optionally filtered by ?connection_id= and limited by ?limit=.
func (h *ProxmoxHandler) ListTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	tasks, err := h.db.ListProxmoxTasks(c.Query("connection_id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// ListNodeTasks returns recent tasks for a specific node, optionally limited by ?limit=.
func (h *ProxmoxHandler) ListNodeTasks(c *gin.Context) {
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	tasks, err := h.db.ListProxmoxTasksByNode(node.ConnectionID, node.NodeName, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// ─── Backup jobs ──────────────────────────────────────────────────────────────

// ListBackupJobs returns backup job configurations, optionally filtered by ?connection_id=.
func (h *ProxmoxHandler) ListBackupJobs(c *gin.Context) {
	jobs, err := h.db.ListProxmoxBackupJobs(c.Query("connection_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// ─── Backup runs ──────────────────────────────────────────────────────────────

// ListBackupRuns returns the latest backup result per VM, optionally filtered by ?connection_id=.
func (h *ProxmoxHandler) ListBackupRuns(c *gin.Context) {
	runs, err := h.db.ListProxmoxBackupRuns(c.Query("connection_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runs)
}

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
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, lang))
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

// ─── Task log viewer ──────────────────────────────────────────────────────────

// GetTaskLog proxies GET /nodes/{node}/tasks/{upid}/log from PVE.
// upid must be URL-encoded if it contains slashes (it typically doesn't).
func (h *ProxmoxHandler) GetTaskLog(c *gin.Context) {
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

	upid := c.Param("upid")
	if upid == "" {
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(errors.CodeMissingParameter, lang))
		return
	}

	secret, conn, err := h.resolveSecret(node.ConnectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	lines, err := client.GetNodeTaskLog(node.NodeName, upid)
	if err != nil {
		log.Printf("proxmox task-log [%s/%s/%s]: %v", conn.Name, node.NodeName, upid, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lines)
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
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, lang))
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

// ─── Disks ────────────────────────────────────────────────────────────────────

// ListNodeDisks returns physical disks for a node identified by its UUID.
func (h *ProxmoxHandler) ListNodeDisks(c *gin.Context) {
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
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, lang))
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
