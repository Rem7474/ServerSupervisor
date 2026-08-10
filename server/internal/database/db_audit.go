package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Audit Logs ==========

// AuditLogFilter narrows GetAuditLogs — every field is optional (zero value =
// unfiltered). Category matches models.AuditCategories()' keys.
type AuditLogFilter struct {
	Category string
	From     *time.Time
	To       *time.Time
}

func (db *DB) CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error) {
	var id int64
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO audit_logs (username, action, host_id, ip_address, details, status, category)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		username, action, hostID, ipAddress, details, status, models.CategorizeAuditAction(action),
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetAuditLogs returns a filtered, paginated page of audit logs, newest first.
func (db *DB) GetAuditLogs(ctx context.Context, limit, offset int, f AuditLogFilter) ([]models.AuditLog, error) {
	query := `SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at, al.category
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE 1=1`
	args := []any{}
	if f.Category != "" {
		args = append(args, f.Category)
		query += ` AND al.category = $` + strconv.Itoa(len(args))
	}
	if f.From != nil {
		args = append(args, *f.From)
		query += ` AND al.created_at >= $` + strconv.Itoa(len(args))
	}
	if f.To != nil {
		args = append(args, *f.To)
		query += ` AND al.created_at <= $` + strconv.Itoa(len(args))
	}
	args = append(args, limit)
	query += ` ORDER BY al.created_at DESC LIMIT $` + strconv.Itoa(len(args))
	args = append(args, offset)
	query += ` OFFSET $` + strconv.Itoa(len(args))

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAuditLogs(rows)
}

func (db *DB) GetAuditLogsByHost(ctx context.Context, hostID string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at, al.category
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.host_id = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAuditLogs(rows)
}

func (db *DB) GetAuditLogsByUser(ctx context.Context, username string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at, al.category
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.username = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAuditLogs(rows)
}

// CleanOldAuditLogs removes audit logs older than retentionDays, regardless
// of category. Used by the admin-triggered manual purge
// (services/settings.CleanupAuditLogs) — the background job purges per
// category instead (see CleanOldAuditLogsByCategory).
func (db *DB) CleanOldAuditLogs(ctx context.Context, retentionDays int) (int64, error) {
	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM audit_logs WHERE created_at < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanOldAuditLogsByCategory removes audit logs of one category older than
// retentionDays — the per-category counterpart to CleanOldAuditLogs, used by
// the hourly background job (internal/background/audit.go).
func (db *DB) CleanOldAuditLogsByCategory(ctx context.Context, category string, retentionDays int) (int64, error) {
	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM audit_logs WHERE category = $1 AND created_at < NOW() - INTERVAL '1 day' * $2`,
		category, retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) UpdateAuditLogStatus(ctx context.Context, id int64, status, details string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE audit_logs
		 SET status = $1,
		     details = COALESCE(NULLIF($2, ''), details)
		 WHERE id = $3`,
		status, details, id,
	)
	return err
}

// CountAuditLogs returns the total number of audit log entries.
func (db *DB) CountAuditLogs(ctx context.Context) (int64, error) {
	var count int64
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count)
	return count, err
}

func scanAuditLogs(rows *sql.Rows) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.Username, &l.Action, &l.HostID, &l.HostName, &l.IPAddress,
			&l.Details, &l.Status, &l.CreatedAt, &l.Category); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
