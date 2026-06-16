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
func (h *NotificationsHandler) GetNotifications(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	items, readAt, err := h.svc.Recent(c.Request.Context(), c.GetString("username"), 30)
	if err != nil {
		respondError(c, err)
		return
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
