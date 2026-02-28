package database

import (
	"github.com/serversupervisor/server/internal/models"
)

// ========== Audit Logs ==========

func (db *DB) CreateAuditLog(username, action, hostID, ipAddress, details, status string) (int64, error) {
	var id int64
	err := db.conn.QueryRow(
		`INSERT INTO audit_logs (username, action, host_id, ip_address, details, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		username, action, hostID, ipAddress, details, status,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (db *DB) GetAuditLogs(limit, offset int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 ORDER BY al.created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.Username, &l.Action, &l.HostID, &l.HostName, &l.IPAddress,
			&l.Details, &l.Status, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (db *DB) GetAuditLogsByHost(hostID string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.host_id = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.Username, &l.Action, &l.HostID, &l.HostName, &l.IPAddress,
			&l.Details, &l.Status, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (db *DB) GetAuditLogsByUser(username string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.username = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.Username, &l.Action, &l.HostID, &l.HostName, &l.IPAddress,
			&l.Details, &l.Status, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// CleanOldAuditLogs removes audit logs older than retentionDays.
func (db *DB) CleanOldAuditLogs(retentionDays int) (int64, error) {
	result, err := db.conn.Exec(
		`DELETE FROM audit_logs WHERE created_at < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) UpdateAuditLogStatus(id int64, status, details string) error {
	_, err := db.conn.Exec(
		`UPDATE audit_logs
		 SET status = $1,
		     details = COALESCE(NULLIF($2, ''), details)
		 WHERE id = $3`,
		status, details, id,
	)
	return err
}

// CountAuditLogs returns the total number of audit log entries.
func (db *DB) CountAuditLogs() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&count)
	return count, err
}
