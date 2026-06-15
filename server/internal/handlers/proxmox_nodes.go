package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// ListNodes returns all nodes, optionally filtered by ?connection_id.
func (h *ProxmoxHandler) ListNodes(c *gin.Context) {
	nodes, err := h.svc.ListNodes(c.Request.Context(), c.Query("connection_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// GetNode returns a single node.
func (h *ProxmoxHandler) GetNode(c *gin.Context) {
	node, err := h.svc.GetNode(c.Request.Context(), c.Param("id"))
	if err != nil {
		renderProxmoxErr(c, err, true) // localized CodeNodeNotFound on 404
		return
	}
	c.JSON(http.StatusOK, node)
}

// GetNodeMetricsSummary returns time-bucketed avg CPU%/RAM% across all Proxmox nodes.
func (h *ProxmoxHandler) GetNodeMetricsSummary(c *gin.Context) {
	hours, bucketMinutes := proxmoxHoursBucket(c)
	summary, err := h.svc.NodeMetricsSummary(c.Request.Context(), hours, bucketMinutes)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetNodeCPUTemperatureHistory returns the source host's CPU temperature history.
func (h *ProxmoxHandler) GetNodeCPUTemperatureHistory(c *gin.Context) {
	history, err := h.svc.NodeCPUTemperatureHistory(c.Request.Context(), c.Param("id"), proxmoxHours(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

// GetNodeFanRPMHistory returns the source host's fan RPM history.
func (h *ProxmoxHandler) GetNodeFanRPMHistory(c *gin.Context) {
	history, err := h.svc.NodeFanRPMHistory(c.Request.Context(), c.Param("id"), proxmoxHours(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

// ListNodeSensorSourceCandidates returns candidate hosts for node sensors.
func (h *ProxmoxHandler) ListNodeSensorSourceCandidates(c *gin.Context) {
	hosts, err := h.svc.NodeSensorSourceCandidates(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, hosts)
}

// UpdateNodeSensorSource sets or clears a shared source host for node sensors.
func (h *ProxmoxHandler) UpdateNodeSensorSource(c *gin.Context) {
	var req struct {
		HostID string `json:"host_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	updated, err := h.svc.UpdateNodeSensorSource(c.Request.Context(), c.Param("id"), req.HostID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// ListNodeDisks returns physical disks for a node identified by its UUID.
func (h *ProxmoxHandler) ListNodeDisks(c *gin.Context) {
	disks, err := h.svc.NodeDisks(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, disks)
}

// ListNodeServices returns all systemd services on a Proxmox node (Sys.Audit).
func (h *ProxmoxHandler) ListNodeServices(c *gin.Context) {
	services, err := h.svc.NodeServices(c.Request.Context(), c.Param("id"))
	if err != nil {
		renderProxmoxErr(c, err, true) // localized CodeNodeNotFound on 404
		return
	}
	c.JSON(http.StatusOK, services)
}

func proxmoxHours(c *gin.Context) int {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	return hours
}

func proxmoxHoursBucket(c *gin.Context) (int, int) {
	bucketMinutes, _ := strconv.Atoi(c.DefaultQuery("bucket_minutes", "5"))
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}
	return proxmoxHours(c), bucketMinutes
}
