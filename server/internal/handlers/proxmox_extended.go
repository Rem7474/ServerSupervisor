package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseVMID converts a Proxmox task object ID string to an integer VMID.
// Returns 0 if the string is not a valid positive integer.
func parseVMID(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// isSecurityPackage returns true when the package origin or section signals a security update.
func isSecurityPackage(origin, section string) bool {
	lOrigin := strings.ToLower(origin)
	lSection := strings.ToLower(section)
	return strings.Contains(lOrigin, "security") || strings.Contains(lSection, "security")
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
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
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
