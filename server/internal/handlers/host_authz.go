package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// requireHostAccess enforces host-level authorization for handlers that accept
// host_id in request payloads (not path params covered by middleware).
func requireHostAccess(c *gin.Context, db *database.DB, hostID string, requiredLevel string) bool {
	role := c.GetString("role")
	if role == models.RoleAdmin {
		return true
	}

	if hostID == "" {
		respondError(c, apperr.Validation("host_id required"))
		return false
	}

	username := c.GetString("username")
	restricted, level, err := db.GetHostAccess(c.Request.Context(), username, hostID)
	if err != nil {
		respondError(c, apperr.Failed("failed to validate host permissions"))
		return false
	}

	// Backward compatibility: users without explicit per-host entries keep role-based access.
	if !restricted {
		return true
	}

	if level == "" {
		respondError(c, apperr.Forbidden("host access denied"))
		return false
	}

	if requiredLevel == "operator" && level != "operator" {
		respondError(c, apperr.Forbidden("operator host permission required"))
		return false
	}

	return true
}
