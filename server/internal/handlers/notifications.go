package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	notifssvc "github.com/serversupervisor/server/internal/services/notifications"
)

// NotificationsHandler also holds db directly, like AlertRulesHandler, solely
// to resolve the caller's hostperm scope (resolveAlertHostScope in
// host_authz.go) for GetNotifications.
type NotificationsHandler struct {
	svc *notifssvc.Service
	db  *database.DB
}

func NewNotificationsHandler(svc *notifssvc.Service, db *database.DB) *NotificationsHandler {
	return &NotificationsHandler{svc: svc, db: db}
}

// GetNotifications returns the most recent browser notification history entries
// (alerts and release trackers) enriched with display metadata, plus the caller's
// server-side read_at timestamp for cross-device unread-count sync.
//
// Admins and users with no host_permissions entries see every notification;
// hostperm-restricted users only see notifications resolvable to one of their
// granted hosts (see resolvableHostID in host_authz.go) — release-tracker and
// Proxmox/synthetic items have no resolvable host and stay hidden from them
// in this MVP.
//
// Optional query params:
//   - limit (1–200, default 30)
//   - severity: "warn" | "crit"
//   - type: "alert_incident" | "release_tracker"
//   - status: "active" | "resolved"
func (h *NotificationsHandler) GetNotifications(c *gin.Context) {
	scope, err := resolveAlertHostScope(c, h.db)
	if err != nil {
		respondError(c, apperr.Failed("failed to validate host permissions"))
		return
	}
	limit := clampQueryInt(c, "limit", 30, 200)
	items, readAt, err := h.svc.Recent(c.Request.Context(), c.GetString("username"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	items = filterNotificationsByScope(items, scope)

	severity := c.Query("severity")
	typeFilter := c.Query("type")
	statusFilter := c.Query("status")

	if severity != "" || typeFilter != "" || statusFilter != "" {
		filtered := items[:0]
		for _, it := range items {
			if severity != "" && it.Severity != severity {
				continue
			}
			if typeFilter != "" {
				switch typeFilter {
				case "release_tracker":
					if it.Type != "release_tracker_detected" && it.Type != "release_tracker_execution" {
						continue
					}
				default:
					if it.Type != typeFilter {
						continue
					}
				}
			}
			if statusFilter == "active" && it.ResolvedAt != nil {
				continue
			}
			if statusFilter == "resolved" && it.ResolvedAt == nil {
				continue
			}
			filtered = append(filtered, it)
		}
		items = filtered
	}

	c.JSON(http.StatusOK, gin.H{"notifications": items, "total": len(items), "read_at": readAt})
}

// filterNotificationsByScope keeps only the notifications an alertHostScope-
// restricted caller is allowed to see.
func filterNotificationsByScope(items []models.NotificationItem, scope alertHostScope) []models.NotificationItem {
	if scope.unrestricted {
		return items
	}
	out := make([]models.NotificationItem, 0, len(items))
	for _, it := range items {
		if scope.allowsHost(resolvableHostID(it.HostID, it.LinkHostID)) {
			out = append(out, it)
		}
	}
	return out
}

// MarkRead persists the current UTC timestamp as the user's "read up to"
// marker for the calling user. Any authenticated user may mark their own
// notification feed read — the marker is keyed by username, so this can't
// affect or leak another user's read state, and it's the harmless write-side
// counterpart to the now hostperm-scoped GetNotifications read.
func (h *NotificationsHandler) MarkRead(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		respondError(c, apperr.Unauthorized("unauthorized"))
		return
	}
	readAt, err := h.svc.MarkRead(c.Request.Context(), username)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"read_at": readAt})
}
