package database

import (
	"database/sql"
	"encoding/json"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Alert Rules ==========

func (db *DB) CreateAlertRule(rule *models.AlertRule) error {
	rule.NormalizeCompatibility()
	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)
	return db.conn.QueryRow(
		`INSERT INTO alert_rules (source_type, host_id, proxmox_scope, metric, operator, threshold, duration_seconds, actions, enabled)
 VALUES ($1,$2,CAST($3 AS JSONB),$4,$5,$6,$7,CAST($8 AS JSONB),$9)
 RETURNING id`,
		rule.SourceType, rule.HostID, string(proxmoxScopeJSON), rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled,
	).Scan(&rule.ID)
}

func (db *DB) UpdateAlertRule(rule *models.AlertRule) error {
	rule.NormalizeCompatibility()
	actionsJSON, _ := json.Marshal(rule.Actions)
	proxmoxScopeJSON, _ := json.Marshal(rule.ProxmoxScope)
	_, err := db.conn.Exec(
		`UPDATE alert_rules SET
source_type = $1,
host_id = $2,
proxmox_scope = CAST($3 AS JSONB),
metric = $4,
operator = $5,
threshold = $6,
duration_seconds = $7,
actions = CAST($8 AS JSONB),
enabled = $9,
updated_at = NOW()
 WHERE id = $10`,
		rule.SourceType, rule.HostID, string(proxmoxScopeJSON), rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled, rule.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (db *DB) GetAlertRules() ([]models.AlertRule, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, source_type, host_id, proxmox_scope, metric, operator, threshold, duration_seconds,
        actions, last_fired, enabled, created_at, updated_at
 FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var name, hostID, sourceType sql.NullString
		var threshold sql.NullFloat64
		var actionsJSON, proxmoxScopeJSON []byte
		var lastFired, updatedAt sql.NullTime

		if err := rows.Scan(
			&r.ID, &name, &sourceType, &hostID, &proxmoxScopeJSON, &r.Metric, &r.Operator, &threshold, &r.DurationSeconds,
			&actionsJSON, &lastFired, &r.Enabled, &r.CreatedAt, &updatedAt,
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
		if threshold.Valid {
			r.Threshold = &threshold.Float64
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
		if r.Actions.Channels == nil {
			r.Actions.Channels = []string{}
		}
		r.NormalizeCompatibility()

		rules = append(rules, r)
	}
	return rules, nil
}

// ========== Alert Incidents ==========

func (db *DB) GetOpenAlertIncident(ruleID int64, hostID string) (*models.AlertIncident, error) {
	var inc models.AlertIncident
	var nullableRuleID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT id, rule_id, host_id, triggered_at, resolved_at, value
 FROM alert_incidents
 WHERE rule_id = $1 AND host_id = $2 AND resolved_at IS NULL
 ORDER BY triggered_at DESC LIMIT 1`,
		ruleID, hostID,
	).Scan(&inc.ID, &nullableRuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value)
	if err != nil {
		return nil, err
	}
	if nullableRuleID.Valid {
		inc.RuleID = &nullableRuleID.Int64
	}
	return &inc, nil
}

// CreateAlertIncident inserts a new alert incident and returns its generated ID.
func (db *DB) CreateAlertIncident(ruleID int64, hostID string, value float64) (int64, error) {
	var id int64
	err := db.conn.QueryRow(
		`INSERT INTO alert_incidents (rule_id, host_id, value) VALUES ($1, $2, $3) RETURNING id`,
		ruleID, hostID, value,
	).Scan(&id)
	return id, err
}

func (db *DB) ResolveAlertIncident(id int64) error {
	_, err := db.conn.Exec(
		`UPDATE alert_incidents SET resolved_at = NOW() WHERE id = $1 AND resolved_at IS NULL`,
		id,
	)
	return err
}

// ResolveOpenAlertIncidentsByRule marks all open incidents for a rule as resolved.
// It returns the number of incidents that were updated.
func (db *DB) ResolveOpenAlertIncidentsByRule(ruleID int64) (int64, error) {
	result, err := db.conn.Exec(
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

func (db *DB) GetAlertIncidents(limit, offset int) ([]models.AlertIncident, error) {
	rows, err := db.conn.Query(
		`SELECT id, rule_id, host_id, triggered_at, resolved_at, value
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
		if err := rows.Scan(&inc.ID, &nullableRuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value); err != nil {
			continue
		}
		if nullableRuleID.Valid {
			inc.RuleID = &nullableRuleID.Int64
		}
		incidents = append(incidents, inc)
	}
	return incidents, nil
}
