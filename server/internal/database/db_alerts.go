package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Alert Rules ==========

func (db *DB) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	rule.NormalizeCompatibility()
	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)
	dockerScopeJSON, _ := json.Marshal(rule.DockerScope)
	return db.conn.QueryRowContext(ctx,
		`INSERT INTO alert_rules (source_type, host_id, proxmox_scope, docker_scope, metric, operator, threshold_warn, threshold_crit, threshold_clear_warn, threshold_clear_crit, duration_seconds, actions, enabled)
 VALUES ($1,$2,CAST($3 AS JSONB),CAST($4 AS JSONB),$5,$6,$7,$8,$9,$10,$11,CAST($12 AS JSONB),$13)
 RETURNING id`,
		rule.SourceType, rule.HostID, string(proxmoxScopeJSON), string(dockerScopeJSON), rule.Metric, rule.Operator, rule.ThresholdWarn, rule.ThresholdCrit, rule.ThresholdClearWarn, rule.ThresholdClearCrit, rule.DurationSeconds, string(actionsJSON), rule.Enabled,
	).Scan(&rule.ID)
}

func (db *DB) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	rule.NormalizeCompatibility()
	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)
	dockerScopeJSON, _ := json.Marshal(rule.DockerScope)
	_, err := db.conn.ExecContext(ctx,
		`UPDATE alert_rules SET
source_type = $1,
host_id = $2,
proxmox_scope = CAST($3 AS JSONB),
docker_scope = CAST($4 AS JSONB),
metric = $5,
operator = $6,
threshold_warn = $7,
threshold_crit = $8,
threshold_clear_warn = $9,
threshold_clear_crit = $10,
duration_seconds = $11,
actions = CAST($12 AS JSONB),
enabled = $13,
updated_at = NOW()
 WHERE id = $14`,
		rule.SourceType, rule.HostID, string(proxmoxScopeJSON), string(dockerScopeJSON), rule.Metric, rule.Operator, rule.ThresholdWarn, rule.ThresholdCrit, rule.ThresholdClearWarn, rule.ThresholdClearCrit, rule.DurationSeconds, string(actionsJSON), rule.Enabled, rule.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (db *DB) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT ar.id, ar.name, ar.source_type, ar.host_id, ar.proxmox_scope, ar.docker_scope, ar.metric, ar.operator,
        ar.threshold_warn, ar.threshold_crit, ar.threshold_clear_warn, ar.threshold_clear_crit,
        ar.duration_seconds, ar.actions, ar.last_fired, ar.enabled, ar.created_at, ar.updated_at,
        COALESCE(ic.active_count, 0)
 FROM alert_rules ar
 LEFT JOIN (
   SELECT rule_id, COUNT(*) AS active_count
   FROM alert_incidents
   WHERE resolved_at IS NULL
   GROUP BY rule_id
 ) ic ON ic.rule_id = ar.id
 ORDER BY ar.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var name, hostID, sourceType sql.NullString
		var thresholdWarn, thresholdCrit, thresholdClearWarn, thresholdClearCrit sql.NullFloat64
		var actionsJSON, proxmoxScopeJSON, dockerScopeJSON []byte
		var lastFired, updatedAt sql.NullTime

		if err := rows.Scan(
			&r.ID, &name, &sourceType, &hostID, &proxmoxScopeJSON, &dockerScopeJSON, &r.Metric, &r.Operator, &thresholdWarn, &thresholdCrit,
			&thresholdClearWarn, &thresholdClearCrit, &r.DurationSeconds,
			&actionsJSON, &lastFired, &r.Enabled, &r.CreatedAt, &updatedAt,
			&r.ActiveIncidentCount,
		); err != nil {
			continue
		}

		if name.Valid {
			r.Name = &name.String
		}
		if hostID.Valid {
			r.HostID = &hostID.String
		}
		if sourceType.Valid {
			r.SourceType = models.AlertSourceType(sourceType.String)
		}
		if thresholdWarn.Valid {
			r.ThresholdWarn = &thresholdWarn.Float64
		}
		if thresholdCrit.Valid {
			r.ThresholdCrit = &thresholdCrit.Float64
		}
		if thresholdClearWarn.Valid {
			r.ThresholdClearWarn = &thresholdClearWarn.Float64
		}
		if thresholdClearCrit.Valid {
			r.ThresholdClearCrit = &thresholdClearCrit.Float64
		}
		if lastFired.Valid {
			r.LastFired = &lastFired.Time
		}
		if updatedAt.Valid {
			r.UpdatedAt = &updatedAt.Time
		}
		if len(actionsJSON) > 0 {
			_ = json.Unmarshal(actionsJSON, &r.Actions)
		}
		if len(proxmoxScopeJSON) > 0 {
			_ = json.Unmarshal(proxmoxScopeJSON, &r.ProxmoxScope)
		}
		if len(dockerScopeJSON) > 0 {
			_ = json.Unmarshal(dockerScopeJSON, &r.DockerScope)
		}
		if r.Actions.Channels == nil {
			r.Actions.Channels = []string{}
		}
		r.NormalizeCompatibility()

		rules = append(rules, r)
	}
	return rules, nil
}

// ========== Alert Incidents ==========

func (db *DB) GetOpenAlertIncident(ctx context.Context, ruleID int64, hostID string) (*models.AlertIncident, error) {
	var inc models.AlertIncident
	var nullableRuleID sql.NullInt64
	err := db.conn.QueryRowContext(ctx, 
		`SELECT id, rule_id, host_id, severity, triggered_at, resolved_at, value
 FROM alert_incidents
 WHERE rule_id = $1 AND host_id = $2 AND resolved_at IS NULL
 ORDER BY triggered_at DESC LIMIT 1`,
		ruleID, hostID,
	).Scan(&inc.ID, &nullableRuleID, &inc.HostID, &inc.Severity, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value)
	if err != nil {
		return nil, err
	}
	if nullableRuleID.Valid {
		inc.RuleID = &nullableRuleID.Int64
	}
	return &inc, nil
}

// ListOpenAlertIncidentsByRule returns all unresolved incidents for a rule.
func (db *DB) ListOpenAlertIncidentsByRule(ctx context.Context, ruleID int64) ([]models.AlertIncident, error) {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT id, rule_id, host_id, severity, triggered_at, resolved_at, value
		 FROM alert_incidents
		 WHERE rule_id = $1 AND resolved_at IS NULL
		 ORDER BY triggered_at DESC`,
		ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	incidents := make([]models.AlertIncident, 0)
	for rows.Next() {
		var inc models.AlertIncident
		var nullableRuleID sql.NullInt64
		if err := rows.Scan(&inc.ID, &nullableRuleID, &inc.HostID, &inc.Severity, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value); err != nil {
			continue
		}
		if nullableRuleID.Valid {
			inc.RuleID = &nullableRuleID.Int64
		}
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

// CreateAlertIncident inserts a new alert incident and returns its generated ID.
func (db *DB) CreateAlertIncident(ctx context.Context, ruleID int64, hostID string, value float64, severity string) (int64, error) {
	var id int64
	if severity == "" {
		severity = "crit"
	}
	err := db.conn.QueryRowContext(ctx, 
		`INSERT INTO alert_incidents (rule_id, host_id, value, severity) VALUES ($1, $2, $3, $4) RETURNING id`,
		ruleID, hostID, value, severity,
	).Scan(&id)
	return id, err
}

func (db *DB) ResolveAlertIncident(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx, 
		`UPDATE alert_incidents SET resolved_at = NOW() WHERE id = $1 AND resolved_at IS NULL`,
		id,
	)
	return err
}

// UpdateAlertIncidentContext refreshes the host/value/severity of an open incident.
// This keeps the active incident aligned with the latest evaluation state.
func (db *DB) UpdateAlertIncidentContext(ctx context.Context, id int64, hostID string, value float64, severity string) error {
	if severity == "" {
		severity = "crit"
	}
	_, err := db.conn.ExecContext(ctx, 
		`UPDATE alert_incidents
		 SET host_id = $2,
		     value = $3,
		     severity = $4
		 WHERE id = $1 AND resolved_at IS NULL`,
		id, hostID, value, severity,
	)
	return err
}

// ResolveOpenAlertIncidentsByRule marks all open incidents for a rule as resolved.
// It returns the number of incidents that were updated.
func (db *DB) ResolveOpenAlertIncidentsByRule(ctx context.Context, ruleID int64) (int64, error) {
	result, err := db.conn.ExecContext(ctx, 
		`UPDATE alert_incidents SET resolved_at = NOW() WHERE rule_id = $1 AND resolved_at IS NULL`,
		ruleID,
	)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (db *DB) GetAlertIncidents(ctx context.Context, limit, offset int) ([]models.AlertIncident, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, rule_id, host_id, severity, triggered_at, resolved_at, value
 FROM alert_incidents ORDER BY triggered_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var incidents []models.AlertIncident
	for rows.Next() {
		var inc models.AlertIncident
		var nullableRuleID sql.NullInt64
		if err := rows.Scan(&inc.ID, &nullableRuleID, &inc.HostID, &inc.Severity, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value); err != nil {
			continue
		}
		if nullableRuleID.Valid {
			inc.RuleID = &nullableRuleID.Int64
		}
		db.enrichDockerIncident(ctx, &inc)
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

// enrichDockerIncident fills LinkHostID and ValueLabel for incidents whose
// host_id is a synthetic Docker identifier (docker:container: / docker:compose:).
func (db *DB) enrichDockerIncident(ctx context.Context, inc *models.AlertIncident) {
	if strings.HasPrefix(inc.HostID, "docker:container:") {
		uuid := strings.TrimPrefix(inc.HostID, "docker:container:")
		var name, state, hostID string
		if err := db.conn.QueryRowContext(ctx,
			`SELECT name, state, host_id FROM docker_containers WHERE id = $1`, uuid,
		).Scan(&name, &state, &hostID); err == nil {
			inc.LinkHostID = hostID
			inc.ValueLabel = state
		}
		return
	}
	if strings.HasPrefix(inc.HostID, "docker:compose:") {
		rest := strings.TrimPrefix(inc.HostID, "docker:compose:")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			inc.LinkHostID = rest[:idx]
		}
	}
}
