package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	notifssvc "github.com/serversupervisor/server/internal/services/notifications"
)

type NotificationsHandler struct {
	svc *notifssvc.Service
}

func NewNotificationsHandler(svc *notifssvc.Service) *NotificationsHandler {
	return &NotificationsHandler{svc: svc}
}

// GetNotifications returns the most recent browser notification history entries
// (alerts and release trackers) enriched with display metadata, plus the caller's
// server-side read_at timestamp for cross-device unread-count sync.
//
// Optional query params:
//   - limit (1–200, default 30)
//   - severity: "warn" | "crit"
//   - type: "alert_incident" | "release_tracker"
//   - status: "active" | "resolved"
func (h *NotificationsHandler) GetNotifications(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	limit := clampQueryInt(c, "limit", 30, 200)
	items, readAt, err := h.svc.Recent(c.Request.Context(), c.GetString("username"), limit)
	if err != nil {
		respondError(c, err)
		return
	}

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

// MarkRead persists the current UTC timestamp as the user's "read up to" marker.
func (h *NotificationsHandler) MarkRead(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
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
