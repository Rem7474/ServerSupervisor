package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "upid is required"})
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

// resolveSecret returns the token secret and connection details for a connection ID.
// It reads the secret from GetEnabledProxmoxConnections (which includes TokenSecret).
func (h *ProxmoxHandler) resolveSecret(connectionID string) (secret string, conn *models.ProxmoxConnection, err error) {
	conns, err := h.db.GetEnabledProxmoxConnections()
	if err != nil {
		return "", nil, err
	}
	for _, co := range conns {
		if co.ID == connectionID {
			secret = co.TokenSecret
			break
		}
	}
	if secret == "" {
		return "", nil, fmt.Errorf("connection not found or disabled")
	}
	c, err := h.db.GetProxmoxConnectionByID(connectionID)
	if err != nil || c == nil {
		return "", nil, fmt.Errorf("failed to load connection")
	}
	return secret, c, nil
}

// ─── Apt refresh ──────────────────────────────────────────────────────────────

// RefreshNodeApt triggers `apt-get update` on a Proxmox node via the PVE API.
// Requires Sys.Modify privilege on the token.
// Returns the task UPID so the frontend can poll the task list for completion.
func (h *ProxmoxHandler) RefreshNodeApt(c *gin.Context) {
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
	upid, err := client.TriggerNodeAptUpdate(node.NodeName)
	if err != nil {
		log.Printf("proxmox apt-refresh [%s/%s]: %v", conn.Name, node.NodeName, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	log.Printf("proxmox apt-refresh [%s/%s]: triggered, upid=%s", conn.Name, node.NodeName, upid)
	c.JSON(http.StatusOK, gin.H{"upid": upid, "message": "apt update lancé sur le nœud"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
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

// validServiceAction is the set of allowed service action verbs.
var validServiceAction = map[string]bool{
	"start": true, "stop": true, "restart": true, "reload": true,
}

// NodeServiceAction proxies a service action to PVE. Requires Sys.Modify.
// Returns the task UPID so the frontend can poll for completion.
func (h *ProxmoxHandler) NodeServiceAction(c *gin.Context) {
	action := c.Param("action")
	if !validServiceAction[action] {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid action %q; allowed: start stop restart reload", action)})
		return
	}

	node, err := h.db.GetProxmoxNode(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	service := c.Param("service")

	secret, conn, err := h.resolveSecret(node.ConnectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	upid, err := client.NodeServiceAction(node.NodeName, service, action)
	if err != nil {
		log.Printf("proxmox service-action [%s/%s] %s %s: %v", conn.Name, node.NodeName, action, service, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	log.Printf("proxmox service-action [%s/%s] %s %s: upid=%s", conn.Name, node.NodeName, action, service, upid)
	c.JSON(http.StatusOK, gin.H{"upid": upid})
}
