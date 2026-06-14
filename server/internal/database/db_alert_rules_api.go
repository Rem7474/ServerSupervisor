package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/serversupervisor/server/internal/models"
)

// alertRuleAPISelectCols is the column list for the alert-rules API CRUD reads
// (no active-incident count; that join lives in GetAlertRules used by the engine).
const alertRuleAPISelectCols = `
id, name, enabled, source_type, host_id, proxmox_scope, docker_scope, metric, operator, threshold_warn, threshold_crit,
threshold_clear_warn, threshold_clear_crit, duration_seconds, actions, last_fired, created_at, updated_at`

// scanAlertRuleAPI scans one alert rule row in alertRuleAPISelectCols order.
func scanAlertRuleAPI(row interface {
	Scan(dest ...interface{}) error
}) (models.AlertRule, error) {
	var rule models.AlertRule
	var name, hostID, sourceType sql.NullString
	var thresholdWarn, thresholdCrit, thresholdClearWarn, thresholdClearCrit sql.NullFloat64
	var actionsJSON, proxmoxScopeJSON, dockerScopeJSON []byte
	var lastFired, updatedAt sql.NullTime

	if err := row.Scan(
		&rule.ID, &name, &rule.Enabled, &sourceType, &hostID, &proxmoxScopeJSON, &dockerScopeJSON, &rule.Metric,
		&rule.Operator, &thresholdWarn, &thresholdCrit, &thresholdClearWarn, &thresholdClearCrit, &rule.DurationSeconds,
		&actionsJSON, &lastFired, &rule.CreatedAt, &updatedAt,
	); err != nil {
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

// ListAlertRulesAPI returns all alert rules for the API list (newest first).
func (db *DB) ListAlertRulesAPI(ctx context.Context) ([]models.AlertRule, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT`+alertRuleAPISelectCols+` FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	rules := []models.AlertRule{}
	for rows.Next() {
		rule, err := scanAlertRuleAPI(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// GetAlertRuleByID returns a single alert rule, or sql.ErrNoRows when absent.
func (db *DB) GetAlertRuleByID(ctx context.Context, id int64) (*models.AlertRule, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT`+alertRuleAPISelectCols+` FROM alert_rules WHERE id = $1`, id)
	rule, err := scanAlertRuleAPI(row)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// exists runs a SELECT EXISTS(...) and returns the boolean result.
func (db *DB) exists(ctx context.Context, query string, args ...interface{}) (bool, error) {
	var ok bool
	if err := db.conn.QueryRowContext(ctx, query, args...).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

// ===== scope existence primitives (used by alert-rule scope validation) =====

func (db *DB) HostExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM hosts WHERE id = $1)`, id)
}

func (db *DB) DockerContainerExists(ctx context.Context, id, hostID string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM docker_containers WHERE id = $1 AND host_id = $2)`, id, hostID)
}

func (db *DB) ComposeProjectExists(ctx context.Context, name, hostID string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM compose_projects WHERE name = $1 AND host_id = $2)`, name, hostID)
}

func (db *DB) ProxmoxConnectionExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_connections WHERE id = $1)`, id)
}

func (db *DB) ProxmoxNodeExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE id = $1)`, id)
}

func (db *DB) ProxmoxStorageExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_storages WHERE id = $1)`, id)
}

func (db *DB) ProxmoxGuestExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_guests WHERE id = $1)`, id)
}

func (db *DB) ProxmoxDiskExists(ctx context.Context, id string) (bool, error) {
	return db.exists(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_disks WHERE id = $1)`, id)
}
