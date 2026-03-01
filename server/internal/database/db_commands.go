package database

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== Remote Commands ==========

// newUUID generates a UUID v4 using crypto/rand — no external dependency.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (db *DB) CreateRemoteCommand(hostID, module, action, target, payload, triggeredBy string, auditLogID *int64) (*models.RemoteCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	if payload == "" {
		payload = "{}"
	}
	id := newUUID()
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`INSERT INTO remote_commands (id, host_id, module, action, target, payload, triggered_by, audit_log_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at`,
		id, hostID, module, action, target, payload, triggeredBy, auditLogID,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	return &cmd, nil
}

func (db *DB) GetPendingRemoteCommands(hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, module, action, target, payload
		 FROM remote_commands WHERE host_id = $1 AND status = 'pending' ORDER BY created_at ASC`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.PendingCommand
	for rows.Next() {
		var c models.PendingCommand
		if err := rows.Scan(&c.ID, &c.Module, &c.Action, &c.Target, &c.Payload); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func (db *DB) UpdateRemoteCommandStatus(id, status, output string) error {
	switch status {
	case "running":
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, started_at = NOW() WHERE id = $2`,
			status, id)
		return err
	default:
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`,
			status, output, id)
		return err
	}
}

func (db *DB) GetRemoteCommandByID(id string) (*models.RemoteCommand, error) {
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE id = $1`, id,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	return &cmd, nil
}

func (db *DB) GetRemoteCommandsByHostAndModule(hostID, module string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 AND module = $2 ORDER BY created_at DESC LIMIT $3`,
		hostID, module, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.RemoteCommand
	for rows.Next() {
		var cmd models.RemoteCommand
		var startedAt, endedAt sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
			&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt); err != nil {
			continue
		}
		if startedAt.Valid {
			cmd.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			cmd.EndedAt = &endedAt.Time
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// GetRecentCommandsByHost returns the most recent remote commands for a host across all modules.
func (db *DB) GetRecentCommandsByHost(hostID string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 ORDER BY created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.RemoteCommand
	for rows.Next() {
		var cmd models.RemoteCommand
		var startedAt, endedAt sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
			&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt); err != nil {
			continue
		}
		if startedAt.Valid {
			cmd.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			cmd.EndedAt = &endedAt.Time
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// RemoteCommandWithHost embeds RemoteCommand and adds a resolved host name for display.
type RemoteCommandWithHost struct {
	models.RemoteCommand
	HostName string `json:"host_name"`
}

// GetAllRemoteCommands returns paginated remote commands for all hosts, joined with host name.
func (db *DB) GetAllRemoteCommands(limit, offset int) ([]RemoteCommandWithHost, error) {
	rows, err := db.conn.Query(`
		SELECT rc.id, rc.host_id, rc.module, rc.action, rc.target, rc.payload,
		       rc.status, rc.output, rc.triggered_by, rc.audit_log_id,
		       rc.created_at, rc.started_at, rc.ended_at,
		       COALESCE(h.name, rc.host_id) AS host_name
		FROM remote_commands rc
		LEFT JOIN hosts h ON h.id = rc.host_id
		ORDER BY rc.created_at DESC
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RemoteCommandWithHost
	for rows.Next() {
		var c RemoteCommandWithHost
		var startedAt, endedAt sql.NullTime
		var auditLogID sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.HostID, &c.Module, &c.Action, &c.Target, &c.Payload,
			&c.Status, &c.Output, &c.TriggeredBy, &auditLogID,
			&c.CreatedAt, &startedAt, &endedAt,
			&c.HostName,
		); err != nil {
			continue
		}
		if auditLogID.Valid {
			c.AuditLogID = &auditLogID.Int64
		}
		if startedAt.Valid {
			c.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			c.EndedAt = &endedAt.Time
		}
		result = append(result, c)
	}
	return result, nil
}

// CreateCompletedRemoteCommand inserts a remote command that has already finished
// (e.g. a command the agent ran autonomously at startup). started_at and ended_at
// are both set to now, bypassing the usual pending→running→completed lifecycle.
func (db *DB) CreateCompletedRemoteCommand(hostID, module, action, target, output, triggeredBy string, status string, auditLogID *int64) error {
	id := newUUID()
	_, err := db.conn.Exec(`
		INSERT INTO remote_commands
		  (id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, started_at, ended_at)
		VALUES ($1,$2,$3,$4,$5,'{}', $6,$7,$8,$9, NOW(), NOW())`,
		id, hostID, module, action, target, status, output, triggeredBy, auditLogID,
	)
	return err
}

// CountAllRemoteCommands returns the total number of remote commands.
func (db *DB) CountAllRemoteCommands() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM remote_commands`).Scan(&count)
	return count, err
}

// GetRecentNotifications returns the latest alert incidents with enriched metadata
// for WebSocket browser notification delivery.
func (db *DB) GetRecentNotifications(limit int) ([]models.NotificationItem, error) {
	rows, err := db.conn.Query(
		`SELECT ai.id, ai.rule_id, ai.host_id,
		        COALESCE(h.name, ai.host_id) AS host_name,
		        COALESCE(ar.name, ar.metric || ' ' || ar.operator || ' ' || CAST(ar.threshold AS TEXT)) AS rule_name,
		        COALESCE(ar.metric, '') AS metric,
		        ai.value, ai.triggered_at, ai.resolved_at,
		        COALESCE(ar.actions->'channels' @> '["browser"]'::jsonb, FALSE) AS browser_notify
		 FROM alert_incidents ai
		 LEFT JOIN alert_rules ar ON ai.rule_id = ar.id
		 LEFT JOIN hosts h ON ai.host_id = h.id
		 ORDER BY ai.triggered_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.NotificationItem
	for rows.Next() {
		var item models.NotificationItem
		if err := rows.Scan(
			&item.ID, &item.RuleID, &item.HostID,
			&item.HostName, &item.RuleName, &item.Metric,
			&item.Value, &item.TriggeredAt, &item.ResolvedAt,
			&item.BrowserNotify,
		); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// CleanupStalledCommands marks old pending/running commands as failed and closes linked audit logs.
func (db *DB) CleanupStalledCommands(timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $1
		RETURNING audit_log_id`,
		timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup stalled commands: %w", err)
	}
	defer rows.Close()

	var auditIDs []int64
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		if err := rows.Scan(&auditLogID); err == nil && auditLogID.Valid {
			auditIDs = append(auditIDs, auditLogID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("Cleaned up %d stalled remote commands", count)
		if len(auditIDs) > 0 {
			_, _ = db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs))
		}
	}
	return nil
}

// CleanupHostStalledCommands marks old pending/running commands for a specific host as failed.
func (db *DB) CleanupHostStalledCommands(hostID string, timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE host_id = $1
		  AND status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $2
		RETURNING audit_log_id`,
		hostID, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup host stalled commands: %w", err)
	}
	defer rows.Close()

	var auditIDs []int64
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		if err := rows.Scan(&auditLogID); err == nil && auditLogID.Valid {
			auditIDs = append(auditIDs, auditLogID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("Cleaned up %d stalled commands for host %s", count, hostID)
		if len(auditIDs) > 0 {
			_, _ = db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs))
		}
	}
	return nil
}
