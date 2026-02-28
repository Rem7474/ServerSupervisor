package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AlertHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAlertHandler(db *database.DB, cfg *config.Config) *AlertHandler {
	return &AlertHandler{db: db, cfg: cfg}
}

func (h *AlertHandler) ListRules(c *gin.Context) {
	if c.GetString("role") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rules, err := h.db.GetAlertRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch rules"})
		return
	}
	if rules == nil {
		rules = []models.AlertRule{}
	}
	c.JSON(http.StatusOK, rules)
}

func (h *AlertHandler) CreateRule(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req struct {
		HostID          *string                `json:"host_id"`
		Metric          string                 `json:"metric" binding:"required"`
		Operator        string                 `json:"operator" binding:"required"`
		Threshold       *float64               `json:"threshold"`
		DurationSeconds int                    `json:"duration_seconds"`
		Channel         string                 `json:"channel" binding:"required"`
		ChannelConfig   map[string]interface{} `json:"channel_config"`
		Enabled         *bool                  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if !isValidMetric(req.Metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric"})
		return
	}
	if !isValidOperator(req.Operator) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator"})
		return
	}
	if req.Metric != "status_offline" && req.Threshold == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold required"})
		return
	}
	if !isValidChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel"})
		return
	}

	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 60
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Map legacy channel/channel_config to the unified actions field
	actions := models.AlertActions{Channels: []string{req.Channel}}
	if req.ChannelConfig != nil {
		if to, ok := req.ChannelConfig["to"].(string); ok {
			actions.SMTPTo = to
		}
	}

	rule := &models.AlertRule{
		HostID:          req.HostID,
		Metric:          req.Metric,
		Operator:        req.Operator,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Actions:         actions,
		Enabled:         enabled,
	}

	if err := h.db.CreateAlertRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *AlertHandler) UpdateRule(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	var req struct {
		HostID          *string                `json:"host_id"`
		Metric          string                 `json:"metric" binding:"required"`
		Operator        string                 `json:"operator" binding:"required"`
		Threshold       *float64               `json:"threshold"`
		DurationSeconds int                    `json:"duration_seconds"`
		Channel         string                 `json:"channel" binding:"required"`
		ChannelConfig   map[string]interface{} `json:"channel_config"`
		Enabled         *bool                  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if !isValidMetric(req.Metric) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric"})
		return
	}
	if !isValidOperator(req.Operator) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator"})
		return
	}
	if req.Metric != "status_offline" && req.Threshold == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "threshold required"})
		return
	}
	if !isValidChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel"})
		return
	}

	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 60
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Map legacy channel/channel_config to the unified actions field
	actions := models.AlertActions{Channels: []string{req.Channel}}
	if req.ChannelConfig != nil {
		if to, ok := req.ChannelConfig["to"].(string); ok {
			actions.SMTPTo = to
		}
	}

	rule := &models.AlertRule{
		ID:              ruleID,
		HostID:          req.HostID,
		Metric:          req.Metric,
		Operator:        req.Operator,
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Actions:         actions,
		Enabled:         enabled,
	}

	if err := h.db.UpdateAlertRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update rule"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *AlertHandler) DeleteRule(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	if err := h.db.DeleteAlertRule(ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *AlertHandler) ListIncidents(c *gin.Context) {
	if c.GetString("role") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
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

	offset := (page - 1) * limit
	incidents, err := h.db.GetAlertIncidents(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch incidents"})
		return
	}
	if incidents == nil {
		incidents = []models.AlertIncident{}
	}
	c.JSON(http.StatusOK, incidents)
}

func isValidMetric(metric string) bool {
	switch metric {
	case "cpu_percent", "ram_percent", "disk_percent", "status_offline":
		return true
	default:
		return false
	}
}

func isValidOperator(op string) bool {
	switch op {
	case "gt", "lt", "eq":
		return true
	default:
		return false
	}
}

func isValidChannel(channel string) bool {
	switch channel {
	case "notify", "smtp":
		return true
	default:
		return false
	}
}
