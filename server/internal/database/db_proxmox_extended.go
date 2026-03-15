package database

import (
	"database/sql"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ─── Tasks ────────────────────────────────────────────────────────────────────

// UpsertProxmoxTask inserts or updates a task record (keyed on connection_id + upid).
func (db *DB) UpsertProxmoxTask(connectionID, nodeName, upid, taskType, status, userName string, startTime, endTime *time.Time, exitStatus, objectID string) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_tasks
		    (connection_id, node_name, upid, task_type, status, user_name,
		     start_time, end_time, exit_status, object_id, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		ON CONFLICT (connection_id, upid) DO UPDATE SET
		    node_name   = EXCLUDED.node_name,
		    task_type   = EXCLUDED.task_type,
		    status      = EXCLUDED.status,
		    user_name   = EXCLUDED.user_name,
		    start_time  = EXCLUDED.start_time,
		    end_time    = EXCLUDED.end_time,
		    exit_status = EXCLUDED.exit_status,
		    object_id   = EXCLUDED.object_id,
		    last_seen_at = NOW()`,
		connectionID, nodeName, upid, taskType, status, userName,
		startTime, endTime, exitStatus, objectID,
	)
	return err
}

// ListProxmoxTasksByNode returns recent tasks for a node (newest first).
func (db *DB) ListProxmoxTasksByNode(connectionID, nodeName string, limit int) ([]models.ProxmoxTask, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(`
		SELECT id, connection_id, node_name, upid, task_type, status, user_name,
		       start_time, end_time, exit_status, object_id, last_seen_at
		FROM proxmox_tasks
		WHERE connection_id=$1 AND node_name=$2
		ORDER BY start_time DESC NULLS LAST
		LIMIT $3`,
		connectionID, nodeName, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTasks(rows)
}

// ListProxmoxTasks returns tasks across all nodes for a connection (newest first).
// Pass empty connectionID to get tasks from all connections.
func (db *DB) ListProxmoxTasks(connectionID string, limit int) ([]models.ProxmoxTask, error) {
	if limit <= 0 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if connectionID != "" {
		rows, err = db.conn.Query(`
			SELECT id, connection_id, node_name, upid, task_type, status, user_name,
			       start_time, end_time, exit_status, object_id, last_seen_at
			FROM proxmox_tasks
			WHERE connection_id=$1
			ORDER BY start_time DESC NULLS LAST
			LIMIT $2`,
			connectionID, limit)
	} else {
		rows, err = db.conn.Query(`
			SELECT id, connection_id, node_name, upid, task_type, status, user_name,
			       start_time, end_time, exit_status, object_id, last_seen_at
			FROM proxmox_tasks
			ORDER BY start_time DESC NULLS LAST
			LIMIT $1`,
			limit)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanTasks(rows)
}

// GetRecentFailedTaskCount returns the number of failed tasks since the given time.
func (db *DB) GetRecentFailedTaskCount(since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM proxmox_tasks
		WHERE status='stopped' AND exit_status != '' AND exit_status != 'OK'
		  AND start_time >= $1`,
		since).Scan(&count)
	return count, err
}

// DeleteStaleProxmoxTasks removes tasks not seen since the cutoff for a connection.
func (db *DB) DeleteStaleProxmoxTasks(connectionID string, cutoff time.Time) error {
	_, err := db.conn.Exec(
		`DELETE FROM proxmox_tasks WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

func scanTasks(rows *sql.Rows) ([]models.ProxmoxTask, error) {
	var tasks []models.ProxmoxTask
	for rows.Next() {
		var t models.ProxmoxTask
		var startTime, endTime sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.ConnectionID, &t.NodeName, &t.UPID, &t.TaskType,
			&t.Status, &t.UserName, &startTime, &endTime,
			&t.ExitStatus, &t.ObjectID, &t.LastSeenAt,
		); err != nil {
			return nil, err
		}
		if startTime.Valid {
			v := startTime.Time
			t.StartTime = &v
		}
		if endTime.Valid {
			v := endTime.Time
			t.EndTime = &v
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []models.ProxmoxTask{}
	}
	return tasks, rows.Err()
}

// ─── Backup Jobs ──────────────────────────────────────────────────────────────

// UpsertProxmoxBackupJob inserts or updates a backup job record.
func (db *DB) UpsertProxmoxBackupJob(connectionID, jobID string, enabled bool, schedule, storage, mode, compress, vmids, mailTo string) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_backup_jobs
		    (connection_id, job_id, enabled, schedule, storage, mode, compress, vmids, mail_to, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		ON CONFLICT (connection_id, job_id) DO UPDATE SET
		    enabled      = EXCLUDED.enabled,
		    schedule     = EXCLUDED.schedule,
		    storage      = EXCLUDED.storage,
		    mode         = EXCLUDED.mode,
		    compress     = EXCLUDED.compress,
		    vmids        = EXCLUDED.vmids,
		    mail_to      = EXCLUDED.mail_to,
		    last_seen_at = NOW()`,
		connectionID, jobID, enabled, schedule, storage, mode, compress, vmids, mailTo,
	)
	return err
}

// ListProxmoxBackupJobs returns all backup jobs for a connection.
// Pass empty connectionID to get jobs from all connections.
func (db *DB) ListProxmoxBackupJobs(connectionID string) ([]models.ProxmoxBackupJob, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if connectionID != "" {
		rows, err = db.conn.Query(`
			SELECT id, connection_id, job_id, enabled, schedule, storage, mode, compress, vmids, mail_to, last_seen_at
			FROM proxmox_backup_jobs WHERE connection_id=$1 ORDER BY job_id`,
			connectionID)
	} else {
		rows, err = db.conn.Query(`
			SELECT id, connection_id, job_id, enabled, schedule, storage, mode, compress, vmids, mail_to, last_seen_at
			FROM proxmox_backup_jobs ORDER BY job_id`)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var jobs []models.ProxmoxBackupJob
	for rows.Next() {
		var j models.ProxmoxBackupJob
		if err := rows.Scan(
			&j.ID, &j.ConnectionID, &j.JobID, &j.Enabled, &j.Schedule,
			&j.Storage, &j.Mode, &j.Compress, &j.VMIDs, &j.MailTo, &j.LastSeenAt,
		); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if jobs == nil {
		jobs = []models.ProxmoxBackupJob{}
	}
	return jobs, rows.Err()
}

// DeleteStaleProxmoxBackupJobs removes jobs not seen since the cutoff.
func (db *DB) DeleteStaleProxmoxBackupJobs(connectionID string, cutoff time.Time) error {
	_, err := db.conn.Exec(
		`DELETE FROM proxmox_backup_jobs WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

// ─── Backup Runs ──────────────────────────────────────────────────────────────

// UpsertProxmoxBackupRun upserts the latest backup result for a VM.
// UNIQUE on (connection_id, vmid) — one row per VM, always the most recent run.
func (db *DB) UpsertProxmoxBackupRun(connectionID, nodeName string, vmid int, taskUPID, status string, startTime, endTime *time.Time, exitStatus string) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_backup_runs
		    (connection_id, node_name, vmid, task_upid, status, start_time, end_time, exit_status, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (connection_id, vmid) DO UPDATE SET
		    node_name   = EXCLUDED.node_name,
		    task_upid   = EXCLUDED.task_upid,
		    status      = EXCLUDED.status,
		    start_time  = EXCLUDED.start_time,
		    end_time    = EXCLUDED.end_time,
		    exit_status = EXCLUDED.exit_status,
		    last_seen_at = NOW()`,
		connectionID, nodeName, vmid, taskUPID, status, startTime, endTime, exitStatus,
	)
	return err
}

// ListProxmoxBackupRuns returns the latest backup run per VM for a connection.
// Guest name is joined from proxmox_guests when available.
func (db *DB) ListProxmoxBackupRuns(connectionID string) ([]models.ProxmoxBackupRun, error) {
	var (
		rows *sql.Rows
		err  error
	)
	q := `
		SELECT r.id, r.connection_id, r.node_name, r.vmid, r.task_upid,
		       r.status, r.start_time, r.end_time, r.exit_status, r.last_seen_at,
		       COALESCE(g.name, '') AS guest_name
		FROM proxmox_backup_runs r
		LEFT JOIN proxmox_guests g ON g.connection_id = r.connection_id
		    AND g.node_name = r.node_name AND g.vmid = r.vmid`
	if connectionID != "" {
		rows, err = db.conn.Query(q+` WHERE r.connection_id=$1 ORDER BY r.start_time DESC NULLS LAST`, connectionID)
	} else {
		rows, err = db.conn.Query(q + ` ORDER BY r.start_time DESC NULLS LAST`)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []models.ProxmoxBackupRun
	for rows.Next() {
		var r models.ProxmoxBackupRun
		var startTime, endTime sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.ConnectionID, &r.NodeName, &r.VMID, &r.TaskUPID,
			&r.Status, &startTime, &endTime, &r.ExitStatus, &r.LastSeenAt,
			&r.GuestName,
		); err != nil {
			return nil, err
		}
		if startTime.Valid {
			v := startTime.Time
			r.StartTime = &v
		}
		if endTime.Valid {
			v := endTime.Time
			r.EndTime = &v
		}
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []models.ProxmoxBackupRun{}
	}
	return runs, rows.Err()
}

// ─── Disks ────────────────────────────────────────────────────────────────────

// UpsertProxmoxDisk inserts or updates a physical disk record.
func (db *DB) UpsertProxmoxDisk(connectionID, nodeName, devPath, model, serial string, sizeBytes int64, diskType, health string, wearout int) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_disks
		    (connection_id, node_name, dev_path, model, serial, size_bytes, disk_type, health, wearout, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
		ON CONFLICT (connection_id, node_name, dev_path) DO UPDATE SET
		    model        = EXCLUDED.model,
		    serial       = EXCLUDED.serial,
		    size_bytes   = EXCLUDED.size_bytes,
		    disk_type    = EXCLUDED.disk_type,
		    health       = EXCLUDED.health,
		    wearout      = EXCLUDED.wearout,
		    last_seen_at = NOW()`,
		connectionID, nodeName, devPath, model, serial, sizeBytes, diskType, health, wearout,
	)
	return err
}

// ListProxmoxDisksByNode returns all physical disks for a given node.
func (db *DB) ListProxmoxDisksByNode(connectionID, nodeName string) ([]models.ProxmoxDisk, error) {
	rows, err := db.conn.Query(`
		SELECT id, connection_id, node_name, dev_path, model, serial, size_bytes, disk_type, health, wearout, last_seen_at
		FROM proxmox_disks
		WHERE connection_id=$1 AND node_name=$2
		ORDER BY dev_path`,
		connectionID, nodeName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanDisks(rows)
}

// DeleteStaleProxmoxDisks removes disks not seen since the cutoff for a connection + node.
func (db *DB) DeleteStaleProxmoxDisks(connectionID string, cutoff time.Time) error {
	_, err := db.conn.Exec(
		`DELETE FROM proxmox_disks WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

func scanDisks(rows *sql.Rows) ([]models.ProxmoxDisk, error) {
	var disks []models.ProxmoxDisk
	for rows.Next() {
		var d models.ProxmoxDisk
		if err := rows.Scan(
			&d.ID, &d.ConnectionID, &d.NodeName, &d.DevPath, &d.Model, &d.Serial,
			&d.SizeBytes, &d.DiskType, &d.Health, &d.Wearout, &d.LastSeenAt,
		); err != nil {
			return nil, err
		}
		disks = append(disks, d)
	}
	if disks == nil {
		disks = []models.ProxmoxDisk{}
	}
	return disks, rows.Err()
}

// ─── Node update counters ─────────────────────────────────────────────────────

// UpdateProxmoxNodeUpdates sets the pending/security update counts for a node.
func (db *DB) UpdateProxmoxNodeUpdates(connectionID, nodeName string, pendingUpdates, securityUpdates int) error {
	_, err := db.conn.Exec(`
		UPDATE proxmox_nodes
		SET pending_updates=$3, security_updates=$4, last_update_check_at=NOW()
		WHERE connection_id=$1 AND node_name=$2`,
		connectionID, nodeName, pendingUpdates, securityUpdates,
	)
	return err
}
