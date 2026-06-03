package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/alerts"
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
	items, err := h.db.GetRecentNotifications(c.Request.Context(), 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch notifications"})
		return
	}
	if items == nil {
		items = []models.NotificationItem{}
	}

	h.enrichActiveIncidents(c.Request.Context(), items)

	var readAt *time.Time
	if username != "" {
		readAt, _ = h.db.GetNotificationReadAt(c.Request.Context(), username)
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": items,
		"total":         len(items),
		"read_at":       readAt,
	})
}

// enrichActiveIncidents fills CurrentValue / ClearThreshold / Operator on active
// alert incidents so the UI can show the live value and the threshold at which
// the alert will resolve. Rules are fetched once and indexed by ID.
func (h *NotificationsHandler) enrichActiveIncidents(ctx context.Context, items []models.NotificationItem) {
	// Only do the work if at least one active alert incident is present.
	hasActive := false
	for i := range items {
		if items[i].Type == "alert_incident" && items[i].ResolvedAt == nil && items[i].RuleID != nil {
			hasActive = true
			break
		}
	}
	if !hasActive {
		return
	}

	rules, err := h.db.GetAlertRules(ctx)
	if err != nil {
		return
	}
	ruleByID := make(map[int64]models.AlertRule, len(rules))
	for _, r := range rules {
		ruleByID[r.ID] = r
	}

	for i := range items {
		item := &items[i]
		if item.Type != "alert_incident" || item.ResolvedAt != nil || item.RuleID == nil {
			continue
		}
		rule, ok := ruleByID[*item.RuleID]
		if !ok {
			continue
		}

		item.Operator = rule.Operator
		if clear := alerts.ResolveThresholdForSeverity(rule, alerts.AlertSeverity(item.Severity)); clear != nil {
			item.ClearThreshold = clear
		}
		if value, ok := alerts.CurrentIncidentValue(ctx, h.db, rule, item.HostID); ok {
			item.CurrentValue = &value
		}
	}
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
	if err := h.db.UpsertNotificationReadAt(c.Request.Context(), username, readAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update read timestamp"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"read_at": readAt})
}
