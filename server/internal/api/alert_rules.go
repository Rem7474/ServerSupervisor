package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AlertRulesHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAlertRulesHandler(db *database.DB, cfg *config.Config) *AlertRulesHandler {
	return &AlertRulesHandler{
		db:  db,
		cfg: cfg,
	}
}

// ListAlertRules returns all alert rules
func (h *AlertRulesHandler) ListAlertRules(c *gin.Context) {
	query := `
		SELECT id, name, enabled, host_id, metric, operator, threshold, duration_seconds,
		       channels, smtp_to, ntfy_topic, cooldown, last_fired, created_at, updated_at
		FROM alert_rules
		ORDER BY created_at DESC
	`
	
	rows, err := h.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	rules := []models.AlertRule{}
	for rows.Next() {
		var rule models.AlertRule
		var channelsJSON []byte
		var name, smtpTo, ntfyTopic sql.NullString
		var hostID sql.NullString
		var threshold sql.NullFloat64
		var cooldown sql.NullInt32
		var lastFired, updatedAt sql.NullTime

		err := rows.Scan(
			&rule.ID, &name, &rule.Enabled, &hostID, &rule.Metric,
			&rule.Operator, &threshold, &rule.DurationSeconds, &channelsJSON,
			&smtpTo, &ntfyTopic, &cooldown, &lastFired,
			&rule.CreatedAt, &updatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if name.Valid {
			rule.Name = &name.String
		}
		if hostID.Valid {
			rule.HostID = &hostID.String
		}
		if threshold.Valid {
			rule.Threshold = &threshold.Float64
		}
		if smtpTo.Valid {
			rule.SMTPTo = &smtpTo.String
		}
		if ntfyTopic.Valid {
			rule.NtfyTopic = &ntfyTopic.String
		}
		if cooldown.Valid {
			cooldownInt := int(cooldown.Int32)
			rule.Cooldown = &cooldownInt
		}
		if lastFired.Valid {
			rule.LastFired = &lastFired.Time
		}
		if updatedAt.Valid {
			rule.UpdatedAt = &updatedAt.Time
		}

		if len(channelsJSON) > 0 {
			json.Unmarshal(channelsJSON, &rule.Channels)
		} else {
			rule.Channels = []string{}
		}

		rules = append(rules, rule)
	}

	c.JSON(http.StatusOK, rules)
}

// GetAlertRule returns a single alert rule by ID
func (h *AlertRulesHandler) GetAlertRule(c *gin.Context) {
	id := c.Param("id")

	query := `
		SELECT id, name, enabled, host_id, metric, operator, threshold, duration_seconds,
		       channels, smtp_to, ntfy_topic, cooldown, last_fired, created_at, updated_at
		FROM alert_rules
		WHERE id = $1
	`

	var rule models.AlertRule
	var channelsJSON []byte
	var name, smtpTo, ntfyTopic sql.NullString
	var hostID sql.NullString
	var threshold sql.NullFloat64
	var cooldown sql.NullInt32
	var lastFired, updatedAt sql.NullTime

	err := h.db.QueryRow(query, id).Scan(
		&rule.ID, &name, &rule.Enabled, &hostID, &rule.Metric,
		&rule.Operator, &threshold, &rule.DurationSeconds, &channelsJSON,
		&smtpTo, &ntfyTopic, &cooldown, &lastFired,
		&rule.CreatedAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if name.Valid {
		rule.Name = &name.String
	}
	if hostID.Valid {
		rule.HostID = &hostID.String
	}
	if threshold.Valid {
		rule.Threshold = &threshold.Float64
	}
	if smtpTo.Valid {
		rule.SMTPTo = &smtpTo.String
	}
	if ntfyTopic.Valid {
		rule.NtfyTopic = &ntfyTopic.String
	}
	if cooldown.Valid {
		cooldownInt := int(cooldown.Int32)
		rule.Cooldown = &cooldownInt
	}
	if lastFired.Valid {
		rule.LastFired = &lastFired.Time
	}
	if updatedAt.Valid {
		rule.UpdatedAt = &updatedAt.Time
	}
	if len(channelsJSON) > 0 {
		json.Unmarshal(channelsJSON, &rule.Channels)
	} else {
		rule.Channels = []string{}
	}

	c.JSON(http.StatusOK, rule)
}

// CreateAlertRule creates a new alert rule
func (h *AlertRulesHandler) CreateAlertRule(c *gin.Context) {
	var req models.AlertRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate operator
	validOps := map[string]bool{">": true, "<": true, ">=": true, "<=": true}
	if !validOps[req.Operator] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operator"})
		return
	}

	// Validate metric
	validMetrics := map[string]bool{"cpu": true, "memory": true, "disk": true, "load": true}
	if !validMetrics[req.Metric] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid metric"})
		return
	}

	channelsJSON, _ := json.Marshal(req.Channels)

	query := `
		INSERT INTO alert_rules (name, enabled, host_id, metric, operator, threshold, duration_seconds, channels, smtp_to, ntfy_topic, cooldown)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	var rule models.AlertRule
	name := req.Name
	rule.Name = &name
	rule.Enabled = req.Enabled
	rule.HostID = req.HostID
	rule.Metric = req.Metric
	rule.Operator = req.Operator
	threshold := req.Threshold
	rule.Threshold = &threshold
	rule.DurationSeconds = req.Duration
	rule.Channels = req.Channels
	
	if req.SMTPTo != "" {
		rule.SMTPTo = &req.SMTPTo
	}
	if req.NtfyTopic != "" {
		rule.NtfyTopic = &req.NtfyTopic
	}
	if req.Cooldown > 0 {
		rule.Cooldown = &req.Cooldown
	}

	err := h.db.QueryRow(
		query,
		req.Name, req.Enabled, req.HostID, req.Metric, req.Operator,
		req.Threshold, req.Duration, channelsJSON, req.SMTPTo, req.NtfyTopic, req.Cooldown,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// UpdateAlertRule updates an existing alert rule
func (h *AlertRulesHandler) UpdateAlertRule(c *gin.Context) {
	id := c.Param("id")

	var req models.AlertRuleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argCount))
		args = append(args, *req.Name)
		argCount++
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = $"+strconv.Itoa(argCount))
		args = append(args, *req.Enabled)
		argCount++
	}
	if req.HostID != nil {
		updates = append(updates, "host_id = $"+strconv.Itoa(argCount))
		args = append(args, *req.HostID)
		argCount++
	}
	if req.Metric != nil {
		updates = append(updates, "metric = $"+strconv.Itoa(argCount))
		args = append(args, *req.Metric)
		argCount++
	}
	if req.Operator != nil {
		updates = append(updates, "operator = $"+strconv.Itoa(argCount))
		args = append(args, *req.Operator)
		argCount++
	}
	if req.Threshold != nil {
		updates = append(updates, "threshold = $"+strconv.Itoa(argCount))
		args = append(args, *req.Threshold)
		argCount++
	}
	if req.Duration != nil {
		updates = append(updates, "duration_seconds = $"+strconv.Itoa(argCount))
		args = append(args, *req.Duration)
		argCount++
	}
	if req.Channels != nil {
		channelsJSON, _ := json.Marshal(*req.Channels)
		updates = append(updates, "channels = $"+strconv.Itoa(argCount))
		args = append(args, channelsJSON)
		argCount++
	}
	if req.SMTPTo != nil {
		updates = append(updates, "smtp_to = $"+strconv.Itoa(argCount))
		args = append(args, *req.SMTPTo)
		argCount++
	}
	if req.NtfyTopic != nil {
		updates = append(updates, "ntfy_topic = $"+strconv.Itoa(argCount))
		args = append(args, *req.NtfyTopic)
		argCount++
	}
	if req.Cooldown != nil {
		updates = append(updates, "cooldown = $"+strconv.Itoa(argCount))
		args = append(args, *req.Cooldown)
		argCount++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = NOW()")
	args = append(args, id)

	query := "UPDATE alert_rules SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = $" + strconv.Itoa(argCount)

	result, err := h.db.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert rule updated"})
}

// DeleteAlertRule deletes an alert rule
func (h *AlertRulesHandler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")

	result, err := h.db.Exec("DELETE FROM alert_rules WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}

// TestAlertRule tests an alert rule without saving it
func (h *AlertRulesHandler) TestAlertRule(c *gin.Context) {
	var req models.AlertRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simulate evaluation logic
	// In real implementation, this would check current metrics against the rule
	c.JSON(http.StatusOK, gin.H{
		"message": "Test alert would fire if conditions are met",
		"rule": req,
		"timestamp": time.Now(),
	})
}
