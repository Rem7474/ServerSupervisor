package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/serversupervisor/server/internal/alerts"
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

// alertRuleFieldLabel maps Go struct field names to human-readable French labels.
var alertRuleFieldLabel = map[string]string{
	"Name":      "Nom",
	"Metric":    "Métrique",
	"Operator":  "Opérateur",
	"Threshold": "Seuil",
	"Duration":  "Durée",
	"Enabled":   "Activé",
	"HostID":    "Hôte",
}

// alertRuleTagMessage returns a human-readable message for a validator tag.
func alertRuleTagMessage(field, tag string) string {
	label, ok := alertRuleFieldLabel[field]
	if !ok {
		label = field
	}
	switch tag {
	case "required":
		return fmt.Sprintf("Le champ « %s » est obligatoire.", label)
	case "min":
		return fmt.Sprintf("Le champ « %s » est trop court.", label)
	case "max":
		return fmt.Sprintf("Le champ « %s » est trop long.", label)
	case "email":
		return fmt.Sprintf("Le champ « %s » doit être une adresse e-mail valide.", label)
	default:
		return fmt.Sprintf("Le champ « %s » est invalide.", label)
	}
}

// humanizeValidationError converts a go-playground/validator error into a
// single readable string. Falls back to the raw error for non-validation errors.
func humanizeValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	if len(ve) == 1 {
		return alertRuleTagMessage(ve[0].Field(), ve[0].Tag())
	}
	// Multiple errors: list them all.
	msg := "Plusieurs champs sont invalides :"
	for _, fe := range ve {
		msg += " " + alertRuleTagMessage(fe.Field(), fe.Tag()) + ";"
	}
	return msg
}

// scanAlertRule scans a single alert rule row from the DB.
// Expected column order: id, name, enabled, host_id, metric, operator, threshold,
// duration_seconds, actions, last_fired, created_at, updated_at
func scanAlertRule(row interface {
	Scan(dest ...interface{}) error
}) (models.AlertRule, error) {
	var rule models.AlertRule
	var name, hostID sql.NullString
	var threshold sql.NullFloat64
	var actionsJSON []byte
	var lastFired, updatedAt sql.NullTime

	err := row.Scan(
		&rule.ID, &name, &rule.Enabled, &hostID, &rule.Metric,
		&rule.Operator, &threshold, &rule.DurationSeconds,
		&actionsJSON, &lastFired, &rule.CreatedAt, &updatedAt,
	)
	if err != nil {
		return rule, err
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
	if lastFired.Valid {
		rule.LastFired = &lastFired.Time
	}
	if updatedAt.Valid {
		rule.UpdatedAt = &updatedAt.Time
	}
	if len(actionsJSON) > 0 {
		_ = json.Unmarshal(actionsJSON, &rule.Actions)
	}
	if rule.Actions.Channels == nil {
		rule.Actions.Channels = []string{}
	}
	return rule, nil
}

const alertRuleSelectCols = `
	id, name, enabled, host_id, metric, operator, threshold, duration_seconds,
	actions, last_fired, created_at, updated_at`

// ListAlertRules returns all alert rules
func (h *AlertRulesHandler) ListAlertRules(c *gin.Context) {
	rows, err := h.db.Query(`SELECT` + alertRuleSelectCols + `
		FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer func() { _ = rows.Close() }()

	rules := []models.AlertRule{}
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rules = append(rules, rule)
	}
	c.JSON(http.StatusOK, rules)
}

// GetAlertRule returns a single alert rule by ID
func (h *AlertRulesHandler) GetAlertRule(c *gin.Context) {
	id := c.Param("id")
	row := h.db.QueryRow(`SELECT`+alertRuleSelectCols+`
		FROM alert_rules WHERE id = $1`, id)

	rule, err := scanAlertRule(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Alert rule not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// CreateAlertRule creates a new alert rule
func (h *AlertRulesHandler) CreateAlertRule(c *gin.Context) {
	var req models.AlertRuleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}

	validOps := map[string]bool{">": true, "<": true, ">=": true, "<=": true}
	if !validOps[req.Operator] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Opérateur invalide."})
		return
	}
	validMetrics := map[string]bool{
		"cpu": true, "memory": true, "disk": true, "load": true, "heartbeat_timeout": true,
		"disk_smart_status": true, "disk_temperature": true, "proxmox_storage_percent": true,
	}
	if !validMetrics[req.Metric] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Métrique invalide."})
		return
	}

	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}
	actionsJSON, _ := json.Marshal(req.Actions)

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
	rule.Actions = req.Actions

	err := h.db.QueryRow(`
		INSERT INTO alert_rules (name, enabled, host_id, metric, operator, threshold, duration_seconds, actions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CAST($8 AS JSONB))
		RETURNING id, created_at, updated_at`,
		req.Name, req.Enabled, req.HostID, req.Metric, req.Operator,
		req.Threshold, req.Duration, string(actionsJSON),
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
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
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
	if req.Actions != nil {
		if req.Actions.Channels == nil {
			req.Actions.Channels = []string{}
		}
		actionsJSON, _ := json.Marshal(*req.Actions)
		updates = append(updates, "actions = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(actionsJSON))
		argCount++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aucun champ à mettre à jour."})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Règle d'alerte introuvable."})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Règle d'alerte introuvable."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}

// TestAlertRule evaluates a rule against current metrics without saving it.
func (h *AlertRulesHandler) TestAlertRule(c *gin.Context) {
	// Use a dedicated payload for test-only evaluation so draft rules can be
	// validated without requiring fields needed only for persistence (like name).
	var req struct {
		HostID    *string             `json:"host_id"`
		Metric    string              `json:"metric" binding:"required"`
		Operator  string              `json:"operator" binding:"required"`
		Threshold float64             `json:"threshold" binding:"required"`
		Duration  int                 `json:"duration"`
		Actions   models.AlertActions `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": humanizeValidationError(err)})
		return
	}

	threshold := req.Threshold
	rule := models.AlertRule{
		HostID:          req.HostID,
		Metric:          req.Metric,
		Operator:        req.Operator,
		Threshold:       &threshold,
		DurationSeconds: req.Duration,
		Actions:         req.Actions,
		Enabled:         true,
	}

	hosts, err := h.db.GetAllHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hosts"})
		return
	}

	type TestResult struct {
		HostID       string  `json:"host_id"`
		HostName     string  `json:"host_name"`
		CurrentValue float64 `json:"current_value"`
		WouldFire    bool    `json:"would_fire"`
		HasData      bool    `json:"has_data"`
	}

	var results []TestResult
	anyFires := false

	// For display purposes, skip staleness check so we always show the current value.
	ruleNoStaleness := rule
	ruleNoStaleness.DurationSeconds = 0

	for _, host := range hosts {
		if rule.HostID != nil && *rule.HostID != host.ID {
			continue
		}
		// Fetch current value ignoring duration-based staleness (for display).
		value, ok := alerts.GetMetricValue(h.db, host, ruleNoStaleness)
		// would_fire respects the original rule (including staleness).
		_, freshOk := alerts.GetMetricValue(h.db, host, rule)
		wouldFire := ok && freshOk && alerts.MatchRule(rule, host, value)
		if wouldFire {
			anyFires = true
		}
		results = append(results, TestResult{
			HostID:       host.ID,
			HostName:     host.Name,
			CurrentValue: value,
			WouldFire:    wouldFire,
			HasData:      ok,
		})
	}

	if results == nil {
		results = []TestResult{}
	}
	c.JSON(http.StatusOK, gin.H{
		"any_fires":    anyFires,
		"evaluated_at": time.Now(),
		"results":      results,
	})
}

// ListIncidents returns all alert incidents with pagination
func (h *AlertRulesHandler) ListIncidents(c *gin.Context) {
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
