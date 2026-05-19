package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// GetDiskMetrics retourne les dernières métriques de disque pour un hôte
func (h *HostHandler) GetDiskMetrics(c *gin.Context) {
	hostID := c.Param("id")

	metrics, err := h.db.GetLatestDiskMetrics(context.Background(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch disk metrics"})
		return
	}

	if metrics == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// GetDiskMetricsHistory retourne l'historique des métriques de disque pour un point de montage
func (h *HostHandler) GetDiskMetricsHistory(c *gin.Context) {
	hostID := c.Param("id")
	mountPoint := c.Query("mount_point")
	limitStr := c.DefaultQuery("limit", "100")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	if mountPoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mount_point query parameter required"})
		return
	}

	history, err := h.db.GetDiskMetricsHistory(context.Background(), hostID, mountPoint, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch disk metrics history"})
		return
	}

	if history == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetDiskMetricsAggregated retourne l'historique agrégé d'un point de montage avec bucketing adaptatif
func (h *HostHandler) GetDiskMetricsAggregated(c *gin.Context) {
	hostID := c.Param("id")
	mountPoint := c.Query("mount_point")
	if mountPoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mount_point query parameter required"})
		return
	}

	hours, err := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if err != nil || hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}

	points, aggType, err := h.db.GetDiskMetricsAggregated(context.Background(), hostID, mountPoint, hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch disk metrics history"})
		return
	}
	if points == nil {
		points = []models.DiskMetrics{}
	}

	c.JSON(http.StatusOK, gin.H{
		"aggregation_type": aggType,
		"hours":            hours,
		"mount_point":      mountPoint,
		"points":           points,
	})
}

// GetDiskHealth retourne l'état SMART de tous les disques d'un hôte
func (h *HostHandler) GetDiskHealth(c *gin.Context) {
	hostID := c.Param("id")

	health, err := h.db.GetLatestDiskHealth(context.Background(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch disk health"})
		return
	}

	if health == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	c.JSON(http.StatusOK, health)
}
