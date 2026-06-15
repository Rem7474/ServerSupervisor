package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// RefreshNodeApt triggers `apt-get update` on a Proxmox node via the PVE API.
func (h *ProxmoxHandler) RefreshNodeApt(c *gin.Context) {
	upid, err := h.svc.RefreshNodeApt(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"upid": upid, "message": "apt update lancé sur le nœud"})
}

// GetNodeGuestNetworks returns a map of vmid → []GuestNetworkIface for a node's guests.
func (h *ProxmoxHandler) GetNodeGuestNetworks(c *gin.Context) {
	result, err := h.svc.NodeGuestNetworks(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// MigrateGuest migrates a VM or LXC to another node in the same cluster.
// URL params: :id = DB node UUID, :vmid = PVE VMID.
func (h *ProxmoxHandler) MigrateGuest(c *gin.Context) {
	var body struct {
		Target    string `json:"target" binding:"required"`
		GuestType string `json:"guest_type"` // "vm" or "lxc"; defaults to "vm"
		Online    bool   `json:"online"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, apperr.Validation("target requis"))
		return
	}
	var vmid int
	if _, err := fmt.Sscanf(c.Param("vmid"), "%d", &vmid); err != nil || vmid <= 0 {
		respondError(c, apperr.Validation("vmid invalide"))
		return
	}
	upid, err := h.svc.MigrateGuest(c.Request.Context(), c.Param("id"), vmid, body.GuestType, body.Target, body.Online)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"upid": upid, "message": fmt.Sprintf("Migration vers %s lancée", body.Target)})
}

// NodeServiceAction proxies a systemd service action to PVE (start/stop/restart/reload).
func (h *ProxmoxHandler) NodeServiceAction(c *gin.Context) {
	upid, err := h.svc.NodeServiceAction(c.Request.Context(), c.Param("id"), c.Param("service"), c.Param("action"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"upid": upid})
}
