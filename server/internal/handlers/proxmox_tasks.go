package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/errors"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

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
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
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
