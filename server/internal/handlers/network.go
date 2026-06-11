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

// SaveTopologyConfig persists network configuration.
func (h *NetworkHandler) SaveTopologyConfig(c *gin.Context) {
	var cfg models.NetworkTopologyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if err := h.svc.SaveTopologyConfig(c.Request.Context(), &cfg); err != nil {
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
