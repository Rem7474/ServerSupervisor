package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/errors"
)

// ─── Live node status (proxied from PVE, not stored in DB) ───────────────────

// GetNodeStatus proxies GET /nodes/{node}/status from PVE (real-time iowait/swap/rootfs).
func (h *ProxmoxHandler) GetNodeStatus(c *gin.Context) {
	status, err := h.svc.NodeStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// GetNodeRRD proxies GET /nodes/{node}/rrddata from PVE.
// Accepts ?timeframe=hour|day|week|month|year (default: hour).
func (h *ProxmoxHandler) GetNodeRRD(c *gin.Context) {
	timeframe := c.DefaultQuery("timeframe", "hour")
	switch timeframe {
	case "hour", "day", "week", "month", "year":
	default:
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(errors.CodeInvalidTimeframe, lang))
		return
	}
	points, err := h.svc.NodeRRD(c.Request.Context(), c.Param("id"), timeframe)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, points)
}
