package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Backup runs (Restic) ==========

const backupRunColumns = `id, host_id, profile, command_id, triggered_by, status,
	started_at, finished_at, duration_sec, progress_percent, files_done, files_total,
	bytes_done, bytes_total, snapshot_id, snapshot_time, repo_size_bytes, error_message,
	raw_summary, created_at`

func scanBackupRun(scan func(dest ...any) error) (*models.BackupRun, error) {
	var r models.BackupRun
	var commandID sql.NullString
	var finishedAt, snapshotTime sql.NullTime
	var durationSec sql.NullInt64
	var progressPercent sql.NullFloat64
	var filesDone, filesTotal, bytesDone, bytesTotal, repoSizeBytes sql.NullInt64
	var rawSummary sql.NullString

	if err := scan(
		&r.ID, &r.HostID, &r.Profile, &commandID, &r.TriggeredBy, &r.Status,
		&r.StartedAt, &finishedAt, &durationSec, &progressPercent, &filesDone, &filesTotal,
		&bytesDone, &bytesTotal, &r.SnapshotID, &snapshotTime, &repoSizeBytes, &r.ErrorMessage,
		&rawSummary, &r.CreatedAt,
	); err != nil {
		return nil, err
	}

	if commandID.Valid {
		r.CommandID = &commandID.String
	}
	if finishedAt.Valid {
		r.FinishedAt = &finishedAt.Time
	}
	if durationSec.Valid {
		v := int(durationSec.Int64)
		r.DurationSec = &v
	}
	if progressPercent.Valid {
		r.ProgressPercent = &progressPercent.Float64
	}
	if filesDone.Valid {
		r.FilesDone = &filesDone.Int64
	}
	if filesTotal.Valid {
		r.FilesTotal = &filesTotal.Int64
	}
	if bytesDone.Valid {
		r.BytesDone = &bytesDone.Int64
	}
	if bytesTotal.Valid {
		r.BytesTotal = &bytesTotal.Int64
	}
	if snapshotTime.Valid {
		r.SnapshotTime = &snapshotTime.Time
	}
	if repoSizeBytes.Valid {
		r.RepoSizeBytes = &repoSizeBytes.Int64
	}
	if rawSummary.Valid {
		r.RawSummary = &rawSummary.String
	}
	return &r, nil
}

// CreateBackupRun inserts a new backup run and returns it.
func (db *DB) CreateBackupRun(ctx context.Context, r models.BackupRun) (*models.BackupRun, error) {
	status := r.Status
	if status == "" {
		status = "running"
	}
	row := db.conn.QueryRowContext(ctx, `
		INSERT INTO backup_runs (host_id, profile, command_id, triggered_by, status, started_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING `+backupRunColumns,
		r.HostID, r.Profile, r.CommandID, r.TriggeredBy, status, r.StartedAt,
	)
	out, err := scanBackupRun(row.Scan)
	if err != nil {
		return nil, fmt.Errorf("create backup run: %w", err)
	}
	return out, nil
}

// UpdateBackupRun updates all mutable fields of a backup run by id.
func (db *DB) UpdateBackupRun(ctx context.Context, r models.BackupRun) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE backup_runs SET
			command_id = $1, status = $2, finished_at = $3, duration_sec = $4,
			progress_percent = $5, files_done = $6, files_total = $7,
			bytes_done = $8, bytes_total = $9, snapshot_id = $10, snapshot_time = $11,
			repo_size_bytes = $12, error_message = $13, raw_summary = $14
		WHERE id = $15`,
		r.CommandID, r.Status, r.FinishedAt, r.DurationSec,
		r.ProgressPercent, r.FilesDone, r.FilesTotal,
		r.BytesDone, r.BytesTotal, r.SnapshotID, r.SnapshotTime,
		r.RepoSizeBytes, r.ErrorMessage, r.RawSummary,
		r.ID,
	)
	return err
}

// GetBackupRun returns one backup run by id.
func (db *DB) GetBackupRun(ctx context.Context, id string) (*models.BackupRun, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT `+backupRunColumns+` FROM backup_runs WHERE id = $1`, id)
	return scanBackupRun(row.Scan)
}

// GetBackupRunByCommandID returns the backup run linked to a remote command,
// or sql.ErrNoRows if none exists yet.
func (db *DB) GetBackupRunByCommandID(ctx context.Context, commandID string) (*models.BackupRun, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT `+backupRunColumns+` FROM backup_runs WHERE command_id = $1`, commandID)
	return scanBackupRun(row.Scan)
}

// GetLatestBackupRunByHost returns the most recent backup run for a host.
func (db *DB) GetLatestBackupRunByHost(ctx context.Context, hostID string) (*models.BackupRun, error) {
	row := db.conn.QueryRowContext(ctx, `
		SELECT `+backupRunColumns+` FROM backup_runs
		WHERE host_id = $1 ORDER BY started_at DESC LIMIT 1`, hostID)
	return scanBackupRun(row.Scan)
}

// GetLatestSuccessfulBackupRunByHost returns the most recent "ok" backup run
// for a host — used by the alerts engine's restic_backup_age_hours metric,
// which needs the last *successful* run, not just the last attempt.
func (db *DB) GetLatestSuccessfulBackupRunByHost(ctx context.Context, hostID string) (*models.BackupRun, error) {
	row := db.conn.QueryRowContext(ctx, `
		SELECT `+backupRunColumns+` FROM backup_runs
		WHERE host_id = $1 AND status = 'ok' ORDER BY started_at DESC LIMIT 1`, hostID)
	return scanBackupRun(row.Scan)
}

// ListBackupRuns returns a host's backup run history, most recent first.
func (db *DB) ListBackupRuns(ctx context.Context, hostID string, limit int) ([]models.BackupRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.conn.QueryContext(ctx, `
		SELECT `+backupRunColumns+` FROM backup_runs
		WHERE host_id = $1 ORDER BY started_at DESC LIMIT $2`, hostID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []models.BackupRun
	for rows.Next() {
		r, err := scanBackupRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *r)
	}
	return runs, rows.Err()
}

// ListStalledBackupRuns returns runs still "running" whose started_at is
// older than olderThanMinutes — used by the background stall-check job.
func (db *DB) ListStalledBackupRuns(ctx context.Context, olderThanMinutes int) ([]models.BackupRun, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT `+backupRunColumns+` FROM backup_runs
		WHERE status = 'running' AND started_at < NOW() - ($1 || ' minutes')::interval`,
		olderThanMinutes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []models.BackupRun
	for rows.Next() {
		r, err := scanBackupRun(rows.Scan)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *r)
	}
	return runs, rows.Err()
}

// ========== Restic status (passive, periodic) ==========

// UpsertResticStatus stores the latest passive Restic status snapshot for a
// host — reported periodically by the agent, and refreshed out-of-band right
// after a manual run_backup completes.
func (db *DB) UpsertResticStatus(ctx context.Context, hostID string, status *models.ResticStatus) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO restic_status (host_id, installed, last_run_at, last_status, snapshot_id, repo_size_bytes, error_message, source, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (host_id) DO UPDATE SET
			installed       = EXCLUDED.installed,
			last_run_at     = COALESCE(EXCLUDED.last_run_at, restic_status.last_run_at),
			last_status     = EXCLUDED.last_status,
			snapshot_id     = EXCLUDED.snapshot_id,
			repo_size_bytes = EXCLUDED.repo_size_bytes,
			error_message   = EXCLUDED.error_message,
			source          = EXCLUDED.source,
			updated_at      = NOW()`,
		hostID, status.Installed, status.LastRunAt, status.LastStatus, status.SnapshotID,
		status.RepoSizeBytes, status.ErrorMessage, status.Source,
	)
	return err
}

// GetResticStatus returns the latest passive Restic status snapshot for a host.
func (db *DB) GetResticStatus(ctx context.Context, hostID string) (*models.ResticStatus, error) {
	var s models.ResticStatus
	var lastRunAt sql.NullTime
	var repoSizeBytes sql.NullInt64
	err := db.conn.QueryRowContext(ctx, `
		SELECT installed, last_run_at, last_status, snapshot_id, repo_size_bytes, error_message, source
		FROM restic_status WHERE host_id = $1`, hostID,
	).Scan(&s.Installed, &lastRunAt, &s.LastStatus, &s.SnapshotID, &repoSizeBytes, &s.ErrorMessage, &s.Source)
	if err != nil {
		return nil, err
	}
	if lastRunAt.Valid {
		s.LastRunAt = &lastRunAt.Time
	}
	if repoSizeBytes.Valid {
		s.RepoSizeBytes = &repoSizeBytes.Int64
	}
	return &s, nil
}
