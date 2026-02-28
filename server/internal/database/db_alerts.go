package database

import (
	"database/sql"
	"encoding/json"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Alert Rules ==========

func (db *DB) CreateAlertRule(rule *models.AlertRule) error {
	actionsJSON, _ := json.Marshal(rule.Actions)
	return db.conn.QueryRow(
		`INSERT INTO alert_rules (host_id, metric, operator, threshold, duration_seconds, actions, enabled)
		 VALUES ($1,$2,$3,$4,$5,CAST($6 AS JSONB),$7)
		 RETURNING id`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled,
	).Scan(&rule.ID)
}

func (db *DB) UpdateAlertRule(rule *models.AlertRule) error {
	actionsJSON, _ := json.Marshal(rule.Actions)
	_, err := db.conn.Exec(
		`UPDATE alert_rules SET
			host_id = $1,
			metric = $2,
			operator = $3,
			threshold = $4,
			duration_seconds = $5,
			actions = CAST($6 AS JSONB),
			enabled = $7,
			updated_at = NOW()
		 WHERE id = $8`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled, rule.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (db *DB) GetAlertRules() ([]models.AlertRule, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, host_id, metric, operator, threshold, duration_seconds,
		        actions, last_fired, enabled, created_at, updated_at
		 FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var name, hostID sql.NullString
		var threshold sql.NullFloat64
		var actionsJSON []byte
		var lastFired, updatedAt sql.NullTime

		if err := rows.Scan(
			&r.ID, &name, &hostID, &r.Metric, &r.Operator, &threshold, &r.DurationSeconds,
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
			json.Unmarshal(actionsJSON, &r.Actions)
		}
		if r.Actions.Channels == nil {
			r.Actions.Channels = []string{}
		}

		rules = append(rules, r)
	}
	return rules, nil
}

// ========== Alert Incidents ==========

func (db *DB) GetOpenAlertIncident(ruleID int64, hostID string) (*models.AlertIncident, error) {
	var inc models.AlertIncident
	err := db.conn.QueryRow(
		`SELECT id, rule_id, host_id, triggered_at, resolved_at, value
		 FROM alert_incidents
		 WHERE rule_id = $1 AND host_id = $2 AND resolved_at IS NULL
		 ORDER BY triggered_at DESC LIMIT 1`,
		ruleID, hostID,
	).Scan(&inc.ID, &inc.RuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value)
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

func (db *DB) CreateAlertIncident(ruleID int64, hostID string, value float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO alert_incidents (rule_id, host_id, value) VALUES ($1, $2, $3)`,
		ruleID, hostID, value,
	)
	return err
}

func (db *DB) ResolveAlertIncident(id int64) error {
	_, err := db.conn.Exec(
		`UPDATE alert_incidents SET resolved_at = NOW() WHERE id = $1 AND resolved_at IS NULL`,
		id,
	)
	return err
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
	defer rows.Close()

	var incidents []models.AlertIncident
	for rows.Next() {
		var inc models.AlertIncident
		if err := rows.Scan(&inc.ID, &inc.RuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value); err != nil {
			continue
		}
		incidents = append(incidents, inc)
	}
	return incidents, nil
}
