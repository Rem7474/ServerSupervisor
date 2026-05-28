package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

// ListIncidents returns all alert incidents with pagination
func (h *AlertRulesHandler) ListIncidents(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}

	offset := (page - 1) * limit
	incidents, err := h.db.GetAlertIncidents(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch incidents"})
		return
	}
	if incidents == nil {
		incidents = []models.AlertIncident{}
	}
	c.JSON(http.StatusOK, incidents)
}
