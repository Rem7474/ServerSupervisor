package api

import (
	"net/http"

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

// GetNotifications returns the 30 most recent alert incidents enriched with rule and host names.
func (h *NotificationsHandler) GetNotifications(c *gin.Context) {
	items, err := h.db.GetRecentNotifications(30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch notifications"})
		return
	}
	if items == nil {
		items = []models.NotificationItem{}
	}
	c.JSON(http.StatusOK, gin.H{
		"notifications": items,
		"total":         len(items),
	})
}
