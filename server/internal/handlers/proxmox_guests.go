package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

// ListGuests returns all guests with optional filters: connection_id, type (vm|lxc), status.
func (h *ProxmoxHandler) ListGuests(c *gin.Context) {
	guests, err := h.svc.ListGuests(c.Request.Context(), c.Query("connection_id"), c.Query("type"), c.Query("status"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, guests)
}
