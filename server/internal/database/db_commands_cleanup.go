// Package database — stalled-command cleanups. Commands left in pending /
// running state past their TTL (agent crash, restart, network outage…) are
// failed, and the linked audit logs + scheduled tasks are reconciled so the
// UI doesn't show forever-running rows.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lib/pq"
)

// CleanupStalledCommands marks old pending/running commands as failed and closes linked audit logs
// and linked scheduled tasks.
func (db *DB) CleanupStalledCommands(ctx context.Context, timeoutMinutes int) error {
	rows, err := db.conn.QueryContext(ctx, `
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $1
		RETURNING audit_log_id, scheduled_task_id`,
		timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup stalled commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var auditIDs []int64
	var taskIDs []string
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		var scheduledTaskID sql.NullString
		if err := rows.Scan(&auditLogID, &scheduledTaskID); err == nil {
			if auditLogID.Valid {
				auditIDs = append(auditIDs, auditLogID.Int64)
			}
			if scheduledTaskID.Valid {
				taskIDs = append(taskIDs, scheduledTaskID.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		slog.InfoContext(ctx, "Cleaned up stalled remote commands", slog.Int("count", count))
		if len(auditIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update audit logs during cleanup", slog.Int("count", len(auditIDs)), slog.Any("err", err))
			}
		}
		if len(taskIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE scheduled_tasks SET last_run_status = 'failed', last_run_at = NOW()
				WHERE id = ANY($1)`,
				pq.Array(taskIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update scheduled tasks during cleanup", slog.Int("count", len(taskIDs)), slog.Any("err", err))
			}
		}
	}
	return nil
}

// FailRunningCommandsOnAgentReconnect immediately fails all 'running' commands for a host.
// Called when an offline agent reconnects — any command still in 'running' state is from
// the previous crashed session and will never complete.
func (db *DB) FailRunningCommandsOnAgentReconnect(ctx context.Context, hostID string) error {
	rows, err := db.conn.QueryContext(ctx, `
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Agent restarted — command was interrupted',
		    ended_at = NOW()
		WHERE host_id = $1 AND status = 'running'
		RETURNING audit_log_id, scheduled_task_id`,
		hostID)
	if err != nil {
		return fmt.Errorf("failed to fail running commands on reconnect: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var auditIDs []int64
	var taskIDs []string
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		var scheduledTaskID sql.NullString
		if err := rows.Scan(&auditLogID, &scheduledTaskID); err == nil {
			if auditLogID.Valid {
				auditIDs = append(auditIDs, auditLogID.Int64)
			}
			if scheduledTaskID.Valid {
				taskIDs = append(taskIDs, scheduledTaskID.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		safeHostID := strings.ReplaceAll(strings.ReplaceAll(hostID, "\n", ""), "\r", "")
		slog.InfoContext(ctx, "Agent reconnect: failed interrupted running commands", slog.Int("count", count), slog.String("host", safeHostID))
		if len(auditIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE audit_logs SET status = 'failed',
				    details = 'Agent restarted — command was interrupted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update audit logs on reconnect", slog.String("host", safeHostID), slog.Any("err", err))
			}
		}
		if len(taskIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE scheduled_tasks SET last_run_status = 'failed', last_run_at = NOW()
				WHERE id = ANY($1)`,
				pq.Array(taskIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update scheduled tasks on reconnect", slog.String("host", safeHostID), slog.Any("err", err))
			}
		}
	}
	return nil
}

// CleanupHostStalledCommands marks old pending/running commands for a specific host as failed.
func (db *DB) CleanupHostStalledCommands(ctx context.Context, hostID string, timeoutMinutes int) error {
	rows, err := db.conn.QueryContext(ctx, `
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE host_id = $1
		  AND status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $2
		RETURNING audit_log_id, scheduled_task_id`,
		hostID, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup host stalled commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var auditIDs []int64
	var taskIDs []string
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		var scheduledTaskID sql.NullString
		if err := rows.Scan(&auditLogID, &scheduledTaskID); err == nil {
			if auditLogID.Valid {
				auditIDs = append(auditIDs, auditLogID.Int64)
			}
			if scheduledTaskID.Valid {
				taskIDs = append(taskIDs, scheduledTaskID.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		safeHostID := strings.ReplaceAll(hostID, "\n", "")
		safeHostID = strings.ReplaceAll(safeHostID, "\r", "")
		slog.InfoContext(ctx, "Cleaned up stalled commands for host", slog.Int("count", count), slog.String("host", safeHostID))
		if len(auditIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update audit logs for host during cleanup", slog.Int("count", len(auditIDs)), slog.String("host", safeHostID), slog.Any("err", err))
			}
		}
		if len(taskIDs) > 0 {
			if _, err := db.conn.ExecContext(ctx, `
				UPDATE scheduled_tasks SET last_run_status = 'failed', last_run_at = NOW()
				WHERE id = ANY($1)`,
				pq.Array(taskIDs)); err != nil {
				slog.WarnContext(ctx, "failed to update scheduled tasks for host during cleanup", slog.Int("count", len(taskIDs)), slog.String("host", safeHostID), slog.Any("err", err))
			}
		}
	}
	return nil
}
