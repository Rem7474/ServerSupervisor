package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// GetDiskMetrics retourne les dernières métriques de disque pour un hôte.
func (h *HostHandler) GetDiskMetrics(c *gin.Context) {
	metrics, err := h.svc.DiskMetrics(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, metrics)
}

// GetDiskMetricsHistory retourne l'historique des métriques de disque pour un point de montage.
func (h *HostHandler) GetDiskMetricsHistory(c *gin.Context) {
	mountPoint := c.Query("mount_point")
	if mountPoint == "" {
		respondError(c, apperr.Validation("mount_point query parameter required"))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	history, err := h.svc.DiskMetricsHistory(c.Request.Context(), c.Param("id"), mountPoint, limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

// GetDiskMetricsAggregated retourne l'historique agrégé d'un point de montage avec bucketing adaptatif.
func (h *HostHandler) GetDiskMetricsAggregated(c *gin.Context) {
	mountPoint := c.Query("mount_point")
	if mountPoint == "" {
		respondError(c, apperr.Validation("mount_point query parameter required"))
		return
	}
	hours, err := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if err != nil || hours <= 0 {
		hours = 24
	}
	if hours > 8760 {
		hours = 8760
	}
	points, aggType, err := h.svc.DiskMetricsAggregated(c.Request.Context(), c.Param("id"), mountPoint, hours)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"aggregation_type": aggType,
		"hours":            hours,
		"mount_point":      mountPoint,
		"points":           points,
	})
}

// GetDiskHealth retourne l'état SMART de tous les disques d'un hôte.
func (h *HostHandler) GetDiskHealth(c *gin.Context) {
	health, err := h.svc.DiskHealth(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, health)
}
