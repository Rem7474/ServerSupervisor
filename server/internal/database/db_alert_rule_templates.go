package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/serversupervisor/server/internal/models"
)

func (db *DB) CreateAlertRuleTemplate(ctx context.Context, t *models.AlertRuleTemplate) error {
	actionsJSON, _ := json.Marshal(t.Actions)
	return db.conn.QueryRowContext(ctx,
		`INSERT INTO alert_rule_templates (name, metric, operator, threshold_warn, threshold_crit,
		                                   threshold_clear_warn, threshold_clear_crit, duration_seconds, actions, baseline_window_seconds)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,CAST($9 AS JSONB),$10)
		 RETURNING id, created_at, updated_at`,
		t.Name, t.Metric, t.Operator, t.ThresholdWarn, t.ThresholdCrit,
		t.ThresholdClearWarn, t.ThresholdClearCrit, t.DurationSeconds, string(actionsJSON), t.BaselineWindowSeconds,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (db *DB) GetAlertRuleTemplates(ctx context.Context) ([]models.AlertRuleTemplate, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, metric, operator, threshold_warn, threshold_crit,
		        threshold_clear_warn, threshold_clear_crit, duration_seconds, actions::text,
		        created_at, updated_at, baseline_window_seconds
		 FROM alert_rule_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var templates []models.AlertRuleTemplate
	for rows.Next() {
		t, err := scanAlertRuleTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, *t)
	}
	return templates, rows.Err()
}

func (db *DB) GetAlertRuleTemplateByID(ctx context.Context, id int64) (*models.AlertRuleTemplate, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, name, metric, operator, threshold_warn, threshold_crit,
		        threshold_clear_warn, threshold_clear_crit, duration_seconds, actions::text,
		        created_at, updated_at, baseline_window_seconds
		 FROM alert_rule_templates WHERE id = $1`, id)
	return scanAlertRuleTemplate(row)
}

func (db *DB) UpdateAlertRuleTemplate(ctx context.Context, t *models.AlertRuleTemplate) error {
	actionsJSON, _ := json.Marshal(t.Actions)
	return db.conn.QueryRowContext(ctx,
		`UPDATE alert_rule_templates SET
		   name = $1, metric = $2, operator = $3, threshold_warn = $4, threshold_crit = $5,
		   threshold_clear_warn = $6, threshold_clear_crit = $7, duration_seconds = $8,
		   actions = CAST($9 AS JSONB), baseline_window_seconds = $10, updated_at = now()
		 WHERE id = $11
		 RETURNING updated_at`,
		t.Name, t.Metric, t.Operator, t.ThresholdWarn, t.ThresholdCrit,
		t.ThresholdClearWarn, t.ThresholdClearCrit, t.DurationSeconds, string(actionsJSON), t.BaselineWindowSeconds, t.ID,
	).Scan(&t.UpdatedAt)
}

func (db *DB) DeleteAlertRuleTemplate(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM alert_rule_templates WHERE id = $1`, id)
	return err
}

// rowScanner abstracts *sql.Row / *sql.Rows so the two read paths above
// share one scan+JSON-decode implementation.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanAlertRuleTemplate(row rowScanner) (*models.AlertRuleTemplate, error) {
	var t models.AlertRuleTemplate
	var actionsJSON string
	var clearWarn, clearCrit sql.NullFloat64
	var baselineWindowSeconds sql.NullInt64
	if err := row.Scan(&t.ID, &t.Name, &t.Metric, &t.Operator, &t.ThresholdWarn, &t.ThresholdCrit,
		&clearWarn, &clearCrit, &t.DurationSeconds, &actionsJSON, &t.CreatedAt, &t.UpdatedAt, &baselineWindowSeconds); err != nil {
		return nil, err
	}
	if clearWarn.Valid {
		t.ThresholdClearWarn = &clearWarn.Float64
	}
	if clearCrit.Valid {
		t.ThresholdClearCrit = &clearCrit.Float64
	}
	if baselineWindowSeconds.Valid {
		v := int(baselineWindowSeconds.Int64)
		t.BaselineWindowSeconds = &v
	}
	_ = json.Unmarshal([]byte(actionsJSON), &t.Actions)
	return &t, nil
}
