package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id required"})
		return false
	}

	username := c.GetString("username")
	restricted, level, err := db.GetHostAccess(username, hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate host permissions"})
		return false
	}

	// Backward compatibility: users without explicit per-host entries keep role-based access.
	if !restricted {
		return true
	}

	if level == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "host access denied"})
		return false
	}

	if requiredLevel == "operator" && level != "operator" {
		c.JSON(http.StatusForbidden, gin.H{"error": "operator host permission required"})
		return false
	}

	return true
}
