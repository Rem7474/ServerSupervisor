package models

import "time"

// BackupRun is one Restic backup execution — dispatched via
// module=restic action=run_backup (manual trigger or a scheduled task) and
// populated from the linked remote_commands row's terminal result. Never
// contains restic/Swift/SMTP credentials.
type BackupRun struct {
	ID              string     `json:"id" db:"id"`
	HostID          string     `json:"host_id" db:"host_id"`
	Profile         string     `json:"profile,omitempty" db:"profile"`
	CommandID       *string    `json:"command_id,omitempty" db:"command_id"`
	TriggeredBy     string     `json:"triggered_by,omitempty" db:"triggered_by"`
	Status          string     `json:"status" db:"status"` // running | ok | warning | error
	StartedAt       time.Time  `json:"started_at" db:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty" db:"finished_at"`
	DurationSec     *int       `json:"duration_sec,omitempty" db:"duration_sec"`
	ProgressPercent *float64   `json:"progress_percent,omitempty" db:"progress_percent"`
	FilesDone       *int64     `json:"files_done,omitempty" db:"files_done"`
	FilesTotal      *int64     `json:"files_total,omitempty" db:"files_total"`
	BytesDone       *int64     `json:"bytes_done,omitempty" db:"bytes_done"`
	BytesTotal      *int64     `json:"bytes_total,omitempty" db:"bytes_total"`
	SnapshotID      string     `json:"snapshot_id,omitempty" db:"snapshot_id"`
	SnapshotTime    *time.Time `json:"snapshot_time,omitempty" db:"snapshot_time"`
	RepoSizeBytes   *int64     `json:"repo_size_bytes,omitempty" db:"repo_size_bytes"`
	ErrorMessage    string     `json:"error_message,omitempty" db:"error_message"`
	RawSummary      *string    `json:"raw_summary,omitempty" db:"raw_summary"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
}

// BackupStatus is the aggregated view returned by GET .../backup: the latest
// backup_runs row when one exists, else a fallback built from the periodic,
// passive ResticStatus report (for hosts whose backups aren't dispatched by
// ServerSupervisor at all).
type BackupStatus struct {
	LatestRun    *BackupRun    `json:"latest_run,omitempty"`
	PassiveState *ResticStatus `json:"passive_state,omitempty"`
}
