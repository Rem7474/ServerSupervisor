package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	networksvc "github.com/serversupervisor/server/internal/services/network"
)

// NetworkHandler translates HTTP to the network service.
type NetworkHandler struct {
	svc *networksvc.Service
}

func NewNetworkHandler(svc *networksvc.Service) *NetworkHandler {
	return &NetworkHandler{svc: svc}
}

func (h *NetworkHandler) GetNetworkSnapshot(c *gin.Context) {
	snapshot, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// GetTopologyConfig returns persisted network configuration.
func (h *NetworkHandler) GetTopologyConfig(c *gin.Context) {
	cfg, err := h.svc.TopologyConfig(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// SaveTopologyConfig persists network configuration. Operator+ only: the
// route sits in the shared authenticated group (viewers read the same
// config), so the role is enforced here.
func (h *NetworkHandler) SaveTopologyConfig(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("operator role required"))
		return
	}
	var cfg models.NetworkTopologyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if err := h.svc.SaveTopologyConfig(c.Request.Context(), &cfg, c.GetString("username"), c.ClientIP()); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetTopologySnapshot returns topology with config.
func (h *NetworkHandler) GetTopologySnapshot(c *gin.Context) {
	snapshot, err := h.svc.TopologySnapshot(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

// GetIPInventory returns the live, non-persisted Proxmox guest IPs + NPM
// domain mappings shown on the Network page's cards view.
func (h *NetworkHandler) GetIPInventory(c *gin.Context) {
	inventory, err := h.svc.IPInventory(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, inventory)
}
