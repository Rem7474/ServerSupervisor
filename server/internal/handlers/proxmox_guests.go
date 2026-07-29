package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// GetGuestMetricsSummary returns time-bucketed CPU%/RAM% history for a single guest.
func (h *ProxmoxHandler) GetGuestMetricsSummary(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	bucketMinutes, _ := strconv.Atoi(c.DefaultQuery("bucket_minutes", "5"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}
	summary, err := h.svc.GuestMetricsSummary(c.Request.Context(), c.Param("id"), hours, bucketMinutes)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetGuestExposure returns the NPM domains that route to this guest's own
// live IP(s) enriched with aggregated web-log traffic over ?period (default
// 24h) — the single-guest counterpart of GetNodeGuestExposure, for the guest
// detail page.
func (h *ProxmoxHandler) GetGuestExposure(c *gin.Context) {
	raw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(raw)
	if err != nil || period <= 0 {
		respondError(c, apperr.Validation("invalid period (example: 24h, 168h)"))
		return
	}
	exposure, err := h.svc.GuestExposure(c.Request.Context(), c.Param("id"), period)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, exposure)
}

// ListGuests returns all guests with optional filters: connection_id, type (vm|lxc), status.
func (h *ProxmoxHandler) ListGuests(c *gin.Context) {
	guests, err := h.svc.ListGuests(c.Request.Context(), c.Query("connection_id"), c.Query("type"), c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, guests)
}
