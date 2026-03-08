package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// GetScheduledTasks returns all scheduled tasks for a host, including the last command ID.
func (db *DB) GetScheduledTasks(hostID string) ([]models.ScheduledTask, error) {
	rows, err := db.conn.Query(`
		SELECT st.id, st.host_id, st.name, st.module, st.action, st.target, st.payload::text,
		       st.cron_expression, st.enabled, st.last_run_at, st.next_run_at, st.last_run_status,
		       st.created_by, st.created_at,
		       (SELECT rc.id FROM remote_commands rc
		        WHERE rc.scheduled_task_id = st.id
		        ORDER BY rc.created_at DESC LIMIT 1) AS last_command_id
		FROM scheduled_tasks st
		WHERE st.host_id = $1
		ORDER BY st.created_at ASC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.ScheduledTask
	for rows.Next() {
		var t models.ScheduledTask
		var lastRunAt, nextRunAt sql.NullTime
		var lastRunStatus, lastCommandID sql.NullString
		if err := rows.Scan(
			&t.ID, &t.HostID, &t.Name, &t.Module, &t.Action, &t.Target, &t.Payload,
			&t.CronExpression, &t.Enabled, &lastRunAt, &nextRunAt, &lastRunStatus,
			&t.CreatedBy, &t.CreatedAt, &lastCommandID,
		); err != nil {
			return nil, err
		}
		if lastRunAt.Valid {
			t.LastRunAt = &lastRunAt.Time
		}
		if nextRunAt.Valid {
			t.NextRunAt = &nextRunAt.Time
		}
		if lastRunStatus.Valid {
			t.LastRunStatus = &lastRunStatus.String
		}
		if lastCommandID.Valid {
			t.LastCommandID = &lastCommandID.String
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetGlobalScheduledTasks returns all scheduled tasks across all hosts, with host name.
// Used by the global scheduled-tasks view.
func (db *DB) GetGlobalScheduledTasks() ([]models.ScheduledTaskWithHost, error) {
	rows, err := db.conn.Query(`
		SELECT st.id, st.host_id, st.name, st.module, st.action, st.target, st.payload::text,
		       st.cron_expression, st.enabled, st.last_run_at, st.next_run_at, st.last_run_status,
		       st.created_by, st.created_at, h.name
		FROM scheduled_tasks st
		JOIN hosts h ON h.id = st.host_id
		ORDER BY h.name ASC, st.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.ScheduledTaskWithHost
	for rows.Next() {
		var t models.ScheduledTaskWithHost
		var lastRunAt, nextRunAt sql.NullTime
		var lastRunStatus sql.NullString
		if err := rows.Scan(
			&t.ID, &t.HostID, &t.Name, &t.Module, &t.Action, &t.Target, &t.Payload,
			&t.CronExpression, &t.Enabled, &lastRunAt, &nextRunAt, &lastRunStatus,
			&t.CreatedBy, &t.CreatedAt, &t.HostName,
		); err != nil {
			return nil, err
		}
		if lastRunAt.Valid {
			t.LastRunAt = &lastRunAt.Time
		}
		if nextRunAt.Valid {
			t.NextRunAt = &nextRunAt.Time
		}
		if lastRunStatus.Valid {
			t.LastRunStatus = &lastRunStatus.String
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetAllScheduledTasks returns all enabled scheduled tasks across all hosts.
// Used by the scheduler on startup.
func (db *DB) GetAllScheduledTasks() ([]models.ScheduledTask, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, name, module, action, target, payload::text,
		       cron_expression, enabled, last_run_at, next_run_at, last_run_status,
		       created_by, created_at
		FROM scheduled_tasks
		WHERE enabled = TRUE
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScheduledTasks(rows)
}

// CreateScheduledTask inserts a new scheduled task and returns it.
func (db *DB) CreateScheduledTask(t models.ScheduledTask) (*models.ScheduledTask, error) {
	payload := t.Payload
	if payload == "" {
		payload = "{}"
	}
	var out models.ScheduledTask
	var lastRunAt, nextRunAt sql.NullTime
	var lastRunStatus sql.NullString
	err := db.conn.QueryRow(`
		INSERT INTO scheduled_tasks (host_id, name, module, action, target, payload,
		                             cron_expression, enabled, created_by)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)
		RETURNING id, host_id, name, module, action, target, payload::text,
		          cron_expression, enabled, last_run_at, next_run_at, last_run_status,
		          created_by, created_at`,
		t.HostID, t.Name, t.Module, t.Action, t.Target, payload,
		t.CronExpression, t.Enabled, t.CreatedBy,
	).Scan(
		&out.ID, &out.HostID, &out.Name, &out.Module, &out.Action, &out.Target, &out.Payload,
		&out.CronExpression, &out.Enabled, &lastRunAt, &nextRunAt, &lastRunStatus,
		&out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create scheduled task: %w", err)
	}
	if lastRunAt.Valid {
		out.LastRunAt = &lastRunAt.Time
	}
	if nextRunAt.Valid {
		out.NextRunAt = &nextRunAt.Time
	}
	if lastRunStatus.Valid {
		out.LastRunStatus = &lastRunStatus.String
	}
	return &out, nil
}

// UpdateScheduledTask updates mutable fields of a scheduled task.
func (db *DB) UpdateScheduledTask(id string, t models.ScheduledTask) error {
	payload := t.Payload
	if payload == "" {
		payload = "{}"
	}
	_, err := db.conn.Exec(`
		UPDATE scheduled_tasks
		SET name=$1, module=$2, action=$3, target=$4, payload=$5::jsonb,
		    cron_expression=$6, enabled=$7
		WHERE id=$8`,
		t.Name, t.Module, t.Action, t.Target, payload,
		t.CronExpression, t.Enabled, id,
	)
	return err
}

// DeleteScheduledTask removes a scheduled task by ID.
func (db *DB) DeleteScheduledTask(id string) error {
	_, err := db.conn.Exec(`DELETE FROM scheduled_tasks WHERE id=$1`, id)
	return err
}

// UpdateScheduledTaskRun records the result of a task execution and updates next_run_at.
func (db *DB) UpdateScheduledTaskRun(id, status string, lastRunAt, nextRunAt time.Time) error {
	_, err := db.conn.Exec(`
		UPDATE scheduled_tasks
		SET last_run_at=$1, next_run_at=$2, last_run_status=$3
		WHERE id=$4`,
		lastRunAt, nextRunAt, status, id,
	)
	return err
}

// UpdateScheduledTaskStatus updates only last_run_status and last_run_at to NOW().
// Used when a command result arrives (completed/failed) to reflect the final outcome.
func (db *DB) UpdateScheduledTaskStatus(id, status string) error {
	_, err := db.conn.Exec(`
		UPDATE scheduled_tasks SET last_run_status = $1, last_run_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

// SetScheduledTaskNextRun updates only the next_run_at field (used after registration).
func (db *DB) SetScheduledTaskNextRun(id string, nextRunAt time.Time) error {
	_, err := db.conn.Exec(`UPDATE scheduled_tasks SET next_run_at=$1 WHERE id=$2`, nextRunAt, id)
	return err
}

// GetScheduledTask returns a single scheduled task by ID.
func (db *DB) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	var out models.ScheduledTask
	var lastRunAt, nextRunAt sql.NullTime
	var lastRunStatus sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, host_id, name, module, action, target, payload::text,
		       cron_expression, enabled, last_run_at, next_run_at, last_run_status,
		       created_by, created_at
		FROM scheduled_tasks WHERE id=$1`, id,
	).Scan(
		&out.ID, &out.HostID, &out.Name, &out.Module, &out.Action, &out.Target, &out.Payload,
		&out.CronExpression, &out.Enabled, &lastRunAt, &nextRunAt, &lastRunStatus,
		&out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastRunAt.Valid {
		out.LastRunAt = &lastRunAt.Time
	}
	if nextRunAt.Valid {
		out.NextRunAt = &nextRunAt.Time
	}
	if lastRunStatus.Valid {
		out.LastRunStatus = &lastRunStatus.String
	}
	return &out, nil
}

func scanScheduledTasks(rows *sql.Rows) ([]models.ScheduledTask, error) {
	var tasks []models.ScheduledTask
	for rows.Next() {
		var t models.ScheduledTask
		var lastRunAt, nextRunAt sql.NullTime
		var lastRunStatus sql.NullString
		if err := rows.Scan(
			&t.ID, &t.HostID, &t.Name, &t.Module, &t.Action, &t.Target, &t.Payload,
			&t.CronExpression, &t.Enabled, &lastRunAt, &nextRunAt, &lastRunStatus,
			&t.CreatedBy, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		if lastRunAt.Valid {
			t.LastRunAt = &lastRunAt.Time
		}
		if nextRunAt.Valid {
			t.NextRunAt = &nextRunAt.Time
		}
		if lastRunStatus.Valid {
			t.LastRunStatus = &lastRunStatus.String
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
