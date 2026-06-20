// Package database — CRUD operations on remote_commands plus the small UUID
// helper. Notifications enrichment lives in db_notifications.go and stalled
// command cleanups in db_commands_cleanup.go.
package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

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

func (db *DB) CreateRemoteCommand(ctx context.Context, hostID, module, action, target, payload, triggeredBy string, auditLogID *int64) (*models.RemoteCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	if payload == "" {
		payload = "{}"
	}
	id := newUUID()
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx,
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

// ClaimPendingRemoteCommands atomically claims all pending commands for a host
// and marks them as running before returning them. This enforces exactly-once
// delivery semantics across report polling cycles.
func (db *DB) ClaimPendingRemoteCommands(ctx context.Context, hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.QueryContext(ctx,
		`WITH claimed AS (
SELECT id
FROM remote_commands
WHERE host_id = $1 AND status = 'pending'
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
)
UPDATE remote_commands rc
SET status = 'running',
    started_at = COALESCE(rc.started_at, NOW())
FROM claimed
WHERE rc.id = claimed.id
RETURNING rc.id, rc.module, rc.action, rc.target, rc.payload`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

// TouchRunningCommandsActivity bumps last_activity_at for all of a host's running
// commands. Called on every agent report: a report proves the agent is alive, so
// its in-flight commands must not be reaped by the stalled-command cleanup even if
// they run long or stay silent (e.g. a first apt update + CVE enrichment). When the
// agent stops reporting, the bump stops and the reaper eventually fails the rows.
func (db *DB) TouchRunningCommandsActivity(ctx context.Context, hostID string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE remote_commands SET last_activity_at = NOW()
		 WHERE host_id = $1 AND status = 'running'`,
		hostID)
	return err
}

func (db *DB) UpdateRemoteCommandStatus(ctx context.Context, id, status, output string) error {
	switch status {
	case "running":
		_, err := db.conn.ExecContext(ctx,
			`UPDATE remote_commands SET status = $1, started_at = NOW() WHERE id = $2`,
			status, id)
		return err
	default:
		_, err := db.conn.ExecContext(ctx,
			`UPDATE remote_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`,
			status, output, id)
		return err
	}
}

func (db *DB) GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error) {
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	var scheduledTaskID sql.NullString
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at, scheduled_task_id
		 FROM remote_commands WHERE id = $1`, id,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt, &scheduledTaskID)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	if scheduledTaskID.Valid {
		cmd.ScheduledTaskID = &scheduledTaskID.String
	}
	return &cmd, nil
}

// LinkCommandToScheduledTask associates a remote command with the scheduled task that triggered it.
func (db *DB) LinkCommandToScheduledTask(ctx context.Context, commandID, taskID string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE remote_commands SET scheduled_task_id = $1 WHERE id = $2`,
		taskID, commandID)
	return err
}

func (db *DB) GetRemoteCommandsByHostAndModule(ctx context.Context, hostID, module string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 AND module = $2 ORDER BY created_at DESC LIMIT $3`,
		hostID, module, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
func (db *DB) GetRecentCommandsByHost(ctx context.Context, hostID string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 ORDER BY created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

// CommandFilter holds optional server-side filter criteria for GetAllRemoteCommands.
type CommandFilter struct {
	Search string // matches host_name, action, target, triggered_by
	Module string
	Status string
}

// buildCommandFilterWhere builds a WHERE+ORDER+LIMIT clause and its args slice.
// The first two positional args are always LIMIT ($N-1) and OFFSET ($N).
func buildCommandFilterWhere(f CommandFilter, limit, offset int) (string, []any) {
	var conds []string
	var args []any

	if f.Module != "" {
		args = append(args, f.Module)
		conds = append(conds, fmt.Sprintf("rc.module = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("rc.status = $%d", len(args)))
	}
	if f.Search != "" {
		pat := "%" + strings.ToLower(f.Search) + "%"
		args = append(args, pat)
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(LOWER(COALESCE(h.name, rc.host_id)) ILIKE $%d OR LOWER(rc.action) ILIKE $%d OR LOWER(rc.target) ILIKE $%d OR LOWER(rc.triggered_by) ILIKE $%d)",
			n, n, n, n,
		))
	}

	clause := ""
	if len(conds) > 0 {
		clause = "WHERE " + strings.Join(conds, " AND ") + " "
	}

	args = append(args, limit, offset)
	clause += fmt.Sprintf("ORDER BY rc.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	return clause, args
}

// buildCommandCountWhere builds a WHERE clause (no LIMIT/OFFSET) and args for COUNT queries.
func buildCommandCountWhere(f CommandFilter) (string, []any) {
	var conds []string
	var args []any

	if f.Module != "" {
		args = append(args, f.Module)
		conds = append(conds, fmt.Sprintf("rc.module = $%d", len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("rc.status = $%d", len(args)))
	}
	if f.Search != "" {
		pat := "%" + strings.ToLower(f.Search) + "%"
		args = append(args, pat)
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(LOWER(COALESCE(h.name, rc.host_id)) ILIKE $%d OR LOWER(rc.action) ILIKE $%d OR LOWER(rc.target) ILIKE $%d OR LOWER(rc.triggered_by) ILIKE $%d)",
			n, n, n, n,
		))
	}

	if len(conds) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}

// GetAllRemoteCommands returns paginated remote commands for all hosts, joined with host name.
func (db *DB) GetAllRemoteCommands(ctx context.Context, limit, offset int, f CommandFilter) ([]RemoteCommandWithHost, error) {
	where, args := buildCommandFilterWhere(f, limit, offset)
	rows, err := db.conn.QueryContext(ctx, `
		SELECT rc.id, rc.host_id, rc.module, rc.action, rc.target, rc.payload,
		       rc.status, rc.output, rc.triggered_by, rc.audit_log_id,
		       rc.created_at, rc.started_at, rc.ended_at,
		       COALESCE(h.name, rc.host_id) AS host_name
		FROM remote_commands rc
		LEFT JOIN hosts h ON h.id = rc.host_id
		`+where,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
func (db *DB) CreateCompletedRemoteCommand(ctx context.Context, hostID, module, action, target, output, triggeredBy string, status string, auditLogID *int64) error {
	id := newUUID()
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO remote_commands
		  (id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, started_at, ended_at)
		VALUES ($1,$2,$3,$4,$5,'{}', $6,$7,$8,$9, NOW(), NOW())`,
		id, hostID, module, action, target, status, output, triggeredBy, auditLogID,
	)
	return err
}

// CancelRemoteCommand sets a command's status to 'cancelled' when it is still
// pending or running. Returns true if a row was updated.
func (db *DB) CancelRemoteCommand(ctx context.Context, id string) (bool, error) {
	res, err := db.conn.ExecContext(ctx,
		`UPDATE remote_commands SET status = 'cancelled' WHERE id = $1 AND status IN ('pending', 'running')`,
		id,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// CountAllRemoteCommands returns the total number of remote commands matching the filter.
func (db *DB) CountAllRemoteCommands(ctx context.Context, f CommandFilter) (int64, error) {
	where, args := buildCommandCountWhere(f)
	q := `SELECT COUNT(*) FROM remote_commands rc LEFT JOIN hosts h ON h.id = rc.host_id`
	if where != "" {
		q += " " + where
	}
	var count int64
	err := db.conn.QueryRowContext(ctx, q, args...).Scan(&count)
	return count, err
}
