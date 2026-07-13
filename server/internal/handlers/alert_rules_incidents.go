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
		respondError(c, apperr.Forbidden("insufficient permissions"))
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

// ListIncidents returns a page of alert incidents. Admins and users with no
// host_permissions entries see everything; hostperm-restricted users only see
// incidents resolvable to one of their granted hosts (direct agent-host
// incidents plus Docker container/compose incidents, resolved via
// LinkHostID — Proxmox and synthetic incidents have no resolvable host and
// stay admin-only-visible in this MVP). Filtering runs after the DB page is
// fetched, so a restricted user's page can come back with fewer than `limit`
// items — an accepted trade-off for this opt-in, niche RBAC case rather than
// pushing host resolution into SQL.
func (h *AlertRulesHandler) ListIncidents(c *gin.Context) {
	scope, err := resolveAlertHostScope(c, h.db)
	if err != nil {
		respondError(c, apperr.Failed("failed to validate host permissions"))
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
	c.JSON(http.StatusOK, filterAlertIncidentsByScope(incidents, scope))
}

// filterAlertIncidentsByScope keeps only the incidents an alertHostScope-
// restricted caller is allowed to see.
func filterAlertIncidentsByScope(incidents []models.AlertIncident, scope alertHostScope) []models.AlertIncident {
	if scope.unrestricted {
		return incidents
	}
	out := make([]models.AlertIncident, 0, len(incidents))
	for _, inc := range incidents {
		if scope.allowsHost(resolvableHostID(inc.HostID, inc.LinkHostID)) {
			out = append(out, inc)
		}
	}
	return out
}
