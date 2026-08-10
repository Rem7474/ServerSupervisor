package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	discoverysvc "github.com/serversupervisor/server/internal/services/discovery"
)

// DiscoveryHandler translates HTTP to the discovery service.
type DiscoveryHandler struct {
	svc *discoverysvc.Service
}

func NewDiscoveryHandler(svc *discoverysvc.Service) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc}
}

// Scan ping-sweeps a subnet and reports which addresses answered (admin
// only — same guard as host registration, since the point is onboarding).
func (h *DiscoveryHandler) Scan(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	var req models.NetworkScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	results, err := h.svc.Scan(c.Request.Context(), req.CIDR)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}
