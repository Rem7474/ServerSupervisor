package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// ResolveIncident manually closes an open alert incident by ID.
func (h *AlertRulesHandler) ResolveIncident(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, apperr.Validation("invalid incident id"))
		return
	}
	if err := h.svc.ResolveIncident(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "incident resolved"})
}

// ListIncidents returns all alert incidents with pagination.
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
	incidents, err := h.svc.ListIncidents(c.Request.Context(), limit, (page-1)*limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, incidents)
}
