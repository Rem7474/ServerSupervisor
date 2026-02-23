package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetDiskMetrics retourne les dernières métriques de disque pour un hôte
func (h *HostHandler) GetDiskMetrics(c *gin.Context) {
	hostID := c.Param("id")

	metrics, err := h.db.GetLatestDiskMetrics(hostID)
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

	history, err := h.db.GetDiskMetricsHistory(hostID, mountPoint, limit)
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

// GetDiskHealth retourne l'état SMART de tous les disques d'un hôte
func (h *HostHandler) GetDiskHealth(c *gin.Context) {
	hostID := c.Param("id")

	health, err := h.db.GetLatestDiskHealth(hostID)
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
