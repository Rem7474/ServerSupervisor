package handlers

import (
	"strings"

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

// alertHostScope describes which hosts a user's alert-rule/incident/
// notification reads are limited to. It is the list-filtering counterpart to
// requireHostAccess's single allow/deny decision: admins and users with no
// host_permissions rows at all are unrestricted (same backward-compatibility
// rule as requireHostAccess); restricted users only see items resolvable to
// one of their granted hosts.
type alertHostScope struct {
	unrestricted bool
	hosts        map[string]bool
}

func (s alertHostScope) allowsHost(hostID string) bool {
	return s.unrestricted || (hostID != "" && s.hosts[hostID])
}

// resolveAlertHostScope computes the caller's alertHostScope for the
// hostperm-filtered read endpoints (alert rules, incidents, notifications).
func resolveAlertHostScope(c *gin.Context, db *database.DB) (alertHostScope, error) {
	if c.GetString("role") == models.RoleAdmin {
		return alertHostScope{unrestricted: true}, nil
	}
	perms, err := db.ListUserHostPermissions(c.Request.Context(), c.GetString("username"))
	if err != nil {
		return alertHostScope{}, err
	}
	if len(perms) == 0 {
		return alertHostScope{unrestricted: true}, nil
	}
	hosts := make(map[string]bool, len(perms))
	for _, p := range perms {
		hosts[p.HostID] = true
	}
	return alertHostScope{hosts: hosts}, nil
}

// resolvableHostID picks the host an alertHostScope check applies to: the
// enrichment-resolved LinkHostID when present, else the raw HostID — unless
// that's a synthetic (Proxmox/Docker/synthetic-probe) identifier this MVP
// doesn't resolve to a real host, in which case it's treated as unscopable
// and hidden from restricted users.
func resolvableHostID(hostID, linkHostID string) string {
	if linkHostID != "" {
		return linkHostID
	}
	if strings.HasPrefix(hostID, "docker:") || strings.HasPrefix(hostID, "proxmox:") || strings.HasPrefix(hostID, "synthetic:") {
		return ""
	}
	return hostID
}
