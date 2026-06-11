package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/models"
)

// alertRuleTagMessage returns a human-readable message for a validator tag.
func alertRuleTagMessage(field, tag string) string {
	label, ok := alertRuleFieldLabel[field]
	if !ok {
		label = field
	}
	switch tag {
	case "required":
		return fmt.Sprintf("Le champ %s est obligatoire.", label)
	case "min":
		return fmt.Sprintf("Le champ %s est trop court.", label)
	case "max":
		return fmt.Sprintf("Le champ %s est trop long.", label)
	case "email":
		return fmt.Sprintf("Le champ %s doit etre une adresse e-mail valide.", label)
	default:
		return fmt.Sprintf("Le champ %s est invalide.", label)
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
	msg := "Plusieurs champs sont invalides :"
	for _, fe := range ve {
		msg += " " + alertRuleTagMessage(fe.Field(), fe.Tag()) + ";"
	}
	return msg
}

// scanAlertRule scans a single alert rule row from the DB.
// Expected column order: id, name, enabled, source_type, host_id, proxmox_scope,
// metric, operator, threshold_warn, threshold_crit, threshold_clear_warn, threshold_clear_crit,
// duration_seconds, actions, last_fired, created_at, updated_at
func scanAlertRule(row interface {
	Scan(dest ...interface{}) error
}) (models.AlertRule, error) {
	var rule models.AlertRule
	var name, hostID, sourceType sql.NullString
	var thresholdWarn, thresholdCrit, thresholdClearWarn, thresholdClearCrit sql.NullFloat64
	var actionsJSON, proxmoxScopeJSON, dockerScopeJSON []byte
	var lastFired, updatedAt sql.NullTime

	err := row.Scan(
		&rule.ID, &name, &rule.Enabled, &sourceType, &hostID, &proxmoxScopeJSON, &dockerScopeJSON, &rule.Metric,
		&rule.Operator, &thresholdWarn, &thresholdCrit, &thresholdClearWarn, &thresholdClearCrit, &rule.DurationSeconds,
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
	if sourceType.Valid {
		rule.SourceType = models.AlertSourceType(sourceType.String)
	}
	if thresholdWarn.Valid {
		rule.ThresholdWarn = &thresholdWarn.Float64
	}
	if thresholdCrit.Valid {
		rule.ThresholdCrit = &thresholdCrit.Float64
	}
	if thresholdClearWarn.Valid {
		rule.ThresholdClearWarn = &thresholdClearWarn.Float64
	}
	if thresholdClearCrit.Valid {
		rule.ThresholdClearCrit = &thresholdClearCrit.Float64
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
	if len(proxmoxScopeJSON) > 0 {
		_ = json.Unmarshal(proxmoxScopeJSON, &rule.ProxmoxScope)
	}
	if len(dockerScopeJSON) > 0 {
		_ = json.Unmarshal(dockerScopeJSON, &rule.DockerScope)
	}
	if rule.Actions.Channels == nil {
		rule.Actions.Channels = []string{}
	}
	rule.NormalizeCompatibility()
	return rule, nil
}

const alertRuleSelectCols = `
id, name, enabled, source_type, host_id, proxmox_scope, docker_scope, metric, operator, threshold_warn, threshold_crit,
threshold_clear_warn, threshold_clear_crit, duration_seconds, actions, last_fired, created_at, updated_at`

// ListAlertRules returns all alert rules
func (h *AlertRulesHandler) ListAlertRules(c *gin.Context) {
	rows, err := h.db.Query(c.Request.Context(), `SELECT`+alertRuleSelectCols+`
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
	row := h.db.QueryRow(c.Request.Context(), `SELECT`+alertRuleSelectCols+`
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

	req.SourceType = normalizeRuleSourceType(req.SourceType, req.Metric)

	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertActions(&req.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}

	var rule models.AlertRule
	name := req.Name
	rule.Name = &name
	rule.Enabled = req.Enabled
	rule.SourceType = req.SourceType
	rule.HostID = req.HostID
	rule.ProxmoxScope = req.ProxmoxScope
	rule.DockerScope = req.DockerScope
	rule.Metric = req.Metric
	rule.Operator = req.Operator
	rule.ThresholdWarn = &req.ThresholdWarn
	rule.ThresholdCrit = &req.ThresholdCrit
	rule.ThresholdClearWarn = req.ThresholdClearWarn
	rule.ThresholdClearCrit = req.ThresholdClearCrit
	rule.DurationSeconds = req.Duration
	rule.Actions = req.Actions

	if err := rule.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(c.Request.Context(), h.db, rule.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if rule.SourceType == models.AlertSourceDocker {
		if err := validateDockerScopeExists(c.Request.Context(), h.db, rule.DockerScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)
	dockerScopeJSON, _ := json.Marshal(rule.DockerScope)

	err := h.db.QueryRow(c.Request.Context(), `
INSERT INTO alert_rules (name, enabled, source_type, host_id, proxmox_scope, docker_scope, metric, operator, threshold_warn, threshold_crit, threshold_clear_warn, threshold_clear_crit, duration_seconds, actions)
VALUES ($1, $2, $3, $4, CAST($5 AS JSONB), CAST($6 AS JSONB), $7, $8, $9, $10, $11, $12, $13, CAST($14 AS JSONB))
RETURNING id, created_at, updated_at`,
		req.Name, req.Enabled, rule.SourceType, rule.HostID, string(proxmoxScopeJSON), string(dockerScopeJSON), rule.Metric, rule.Operator,
		req.ThresholdWarn, req.ThresholdCrit, req.ThresholdClearWarn, req.ThresholdClearCrit, req.Duration, string(actionsJSON),
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

	row := h.db.QueryRow(c.Request.Context(), `SELECT`+alertRuleSelectCols+`
FROM alert_rules WHERE id = $1`, id)
	existing, err := scanAlertRule(row)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.SourceType != nil && *req.SourceType != existing.SourceType {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Le changement de source_type n'est pas autorise."})
		return
	}

	next := existing
	if req.Name != nil {
		next.Name = req.Name
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.HostID != nil {
		next.HostID = req.HostID
	}
	if req.Metric != nil {
		next.Metric = *req.Metric
	}
	if req.Operator != nil {
		next.Operator = *req.Operator
	}
	if req.ThresholdWarn != nil {
		next.ThresholdWarn = req.ThresholdWarn
	}
	if req.ThresholdCrit != nil {
		next.ThresholdCrit = req.ThresholdCrit
	}
	// Hysteresis fields: nil means "clear" (set to NULL) — always apply
	// so the frontend can explicitly remove them by sending null.
	next.ThresholdClearWarn = req.ThresholdClearWarn
	next.ThresholdClearCrit = req.ThresholdClearCrit
	if req.Duration != nil {
		next.DurationSeconds = *req.Duration
	}
	if req.Actions != nil {
		next.Actions = *req.Actions
	}
	if req.ProxmoxScope != nil {
		next.ProxmoxScope = req.ProxmoxScope
	}
	if req.DockerScope != nil {
		next.DockerScope = req.DockerScope
	}

	if err := validateAlertRuleMetricOperator(next.Metric, next.Operator); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateAlertActions(&next.Actions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := next.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if next.SourceType == models.AlertSourceProxmox {
		if err := validateProxmoxScopeExists(c.Request.Context(), h.db, next.ProxmoxScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	if next.SourceType == models.AlertSourceDocker {
		if err := validateDockerScopeExists(c.Request.Context(), h.db, next.DockerScope); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	actionsJSON, _ := json.Marshal(next.Actions)
	proxmoxScopeJSON, _ := json.Marshal(next.ProxmoxScope)
	dockerScopeJSON, _ := json.Marshal(next.DockerScope)

	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argCount))
		args = append(args, *next.Name)
		argCount++
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = $"+strconv.Itoa(argCount))
		args = append(args, next.Enabled)
		argCount++
	}
	if req.HostID != nil {
		updates = append(updates, "host_id = $"+strconv.Itoa(argCount))
		args = append(args, next.HostID)
		argCount++
	}
	if req.ProxmoxScope != nil {
		updates = append(updates, "proxmox_scope = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(proxmoxScopeJSON))
		argCount++
	}
	if req.DockerScope != nil {
		updates = append(updates, "docker_scope = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(dockerScopeJSON))
		argCount++
	}
	if req.Metric != nil {
		if !validAlertMetrics[next.Metric] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Metrique invalide."})
			return
		}
		updates = append(updates, "metric = $"+strconv.Itoa(argCount))
		args = append(args, next.Metric)
		argCount++
	}
	if req.Operator != nil {
		if !validAlertOperators[next.Operator] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Operateur invalide."})
			return
		}
		updates = append(updates, "operator = $"+strconv.Itoa(argCount))
		args = append(args, next.Operator)
		argCount++
	}
	if req.ThresholdWarn != nil {
		updates = append(updates, "threshold_warn = $"+strconv.Itoa(argCount))
		args = append(args, *next.ThresholdWarn)
		argCount++
	}
	if req.ThresholdCrit != nil {
		updates = append(updates, "threshold_crit = $"+strconv.Itoa(argCount))
		args = append(args, *next.ThresholdCrit)
		argCount++
	}
	updates = append(updates, "threshold_clear_warn = $"+strconv.Itoa(argCount))
	args = append(args, next.ThresholdClearWarn) // nil → SQL NULL (clears hysteresis)
	argCount++
	updates = append(updates, "threshold_clear_crit = $"+strconv.Itoa(argCount))
	args = append(args, next.ThresholdClearCrit) // nil → SQL NULL (clears hysteresis)
	argCount++
	if req.Duration != nil {
		updates = append(updates, "duration_seconds = $"+strconv.Itoa(argCount))
		args = append(args, next.DurationSeconds)
		argCount++
	}
	if req.Actions != nil {
		updates = append(updates, "actions = CAST($"+strconv.Itoa(argCount)+" AS JSONB)")
		args = append(args, string(actionsJSON))
		argCount++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Aucun champ a mettre a jour."})
		return
	}

	updates = append(updates, "updated_at = NOW()")
	args = append(args, id)

	query := "UPDATE alert_rules SET " + updates[0]
	for i := 1; i < len(updates); i++ {
		query += ", " + updates[i]
	}
	query += " WHERE id = $" + strconv.Itoa(argCount)

	result, err := h.db.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	if req.Enabled != nil && !next.Enabled {
		ruleID, parseErr := strconv.ParseInt(id, 10, 64)
		if parseErr == nil {
			if _, err := h.db.ResolveOpenAlertIncidentsByRule(c.Request.Context(), ruleID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Regle mise a jour, mais echec de resolution des incidents ouverts."})
				return
			}
		}
	} else {
		// Thresholds or hysteresis may have changed: immediately resolve any open
		// incidents whose stored value no longer meets the (new) firing condition.
		go alerts.ResolveStaleIncidentsForRule(context.Background(), h.db, next)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule updated"})
}

// DeleteAlertRule deletes an alert rule
func (h *AlertRulesHandler) DeleteAlertRule(c *gin.Context) {
	id := c.Param("id")
	result, err := h.db.Exec(c.Request.Context(), "DELETE FROM alert_rules WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Regle d'alerte introuvable."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert rule deleted"})
}
