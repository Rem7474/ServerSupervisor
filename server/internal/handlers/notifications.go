package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type NotificationsHandler struct {
	db *database.DB
}

func NewNotificationsHandler(db *database.DB) *NotificationsHandler {
	return &NotificationsHandler{db: db}
}

// GetNotifications returns the most recent browser notification history entries
// (alerts and release trackers) enriched with display metadata.
// It also includes the caller's server-side read_at timestamp for cross-device unread-count sync.
func (h *NotificationsHandler) GetNotifications(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	username := c.GetString("username")
	items, err := h.db.GetRecentNotifications(30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch notifications"})
		return
	}
	if items == nil {
		items = []models.NotificationItem{}
	}

	var readAt *time.Time
	if username != "" {
		readAt, _ = h.db.GetNotificationReadAt(username)
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": items,
		"total":         len(items),
		"read_at":       readAt,
	})
}

// MarkRead persists the current UTC timestamp as the user's "read up to" marker.
// All notifications triggered before this moment are treated as read on every device.
func (h *NotificationsHandler) MarkRead(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	username := c.GetString("username")
	if username == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	readAt := time.Now().UTC()
	if err := h.db.UpsertNotificationReadAt(username, readAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update read timestamp"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"read_at": readAt})
}
