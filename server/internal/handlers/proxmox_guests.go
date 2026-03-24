package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// GetGuestMetricsSummary returns time-bucketed CPU%/RAM% history for a single guest.
// Used by HostDetailView when metrics_source=proxmox to populate the trend charts.
func (h *ProxmoxHandler) GetGuestMetricsSummary(c *gin.Context) {
	guestID := c.Param("id")
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

	summary, err := h.db.GetProxmoxGuestMetricsSummary(guestID, hours, bucketMinutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if summary == nil {
		summary = []models.ProxmoxNodeMetricsSummary{}
	}
	c.JSON(http.StatusOK, summary)
}

// ListGuests returns all guests with optional filters: connection_id, type (vm|lxc), status.
func (h *ProxmoxHandler) ListGuests(c *gin.Context) {
	guests, err := h.db.ListProxmoxGuests(
		c.Query("connection_id"),
		c.Query("type"),
		c.Query("status"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, guests)
}
