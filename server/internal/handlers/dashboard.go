package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	dashboardsvc "github.com/serversupervisor/server/internal/services/dashboard"
)

// DashboardHandler translates HTTP to the dashboard service.
type DashboardHandler struct {
	svc *dashboardsvc.Service
}

func NewDashboardHandler(svc *dashboardsvc.Service) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// Attention returns the aggregated "needs attention" feed (see
// internal/services/dashboard for what it aggregates and why).
func (h *DashboardHandler) Attention(c *gin.Context) {
	items := h.svc.Attention(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"items": items})
}
