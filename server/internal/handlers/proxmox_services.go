package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

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

// ─── Guest network interfaces ─────────────────────────────────────────────────

// GetNodeGuestNetworks returns a map of vmid → []GuestNetworkIface for all guests of a node.
// VM interfaces are fetched via the QEMU guest agent (errors are silently skipped).
// LXC interfaces are fetched natively (always available).
func (h *ProxmoxHandler) GetNodeGuestNetworks(c *gin.Context) {
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

	guests, err := h.db.ListProxmoxGuestsByNode(node.ConnectionID, node.NodeName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)

	result := make(map[int][]proxmoxclient.GuestNetworkIface)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, g := range guests {
		if g.Status != "running" {
			continue
		}
		wg.Add(1)
		go func(guest models.ProxmoxGuest) {
			defer wg.Done()
			var ifaces []proxmoxclient.GuestNetworkIface
			var ferr error
			if guest.GuestType == "vm" {
				ifaces, ferr = client.GetVMNetworkInterfaces(node.NodeName, guest.VMID)
			} else {
				ifaces, ferr = client.GetLXCInterfaces(node.NodeName, guest.VMID)
			}
			if ferr != nil {
				return // agent not running or no permission — skip silently
			}
			if len(ifaces) > 0 {
				mu.Lock()
				result[guest.VMID] = ifaces
				mu.Unlock()
			}
		}(g)
	}

	wg.Wait()
	c.JSON(http.StatusOK, result)
}

// ─── Services (systemd) ────────────────────────────────────────────────────────

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
