package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/serversupervisor/server/internal/models"
)

// ListMaintenanceWindows returns every maintenance window (global view),
// newest first, with the host name joined in for host-scoped windows.
func (db *DB) ListMaintenanceWindows(ctx context.Context) ([]models.MaintenanceWindow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT mw.id, mw.host_id, h.name, mw.reason, mw.starts_at, mw.ends_at, mw.created_by, mw.created_at
		FROM maintenance_windows mw
		LEFT JOIN hosts h ON h.id = mw.host_id
		ORDER BY mw.starts_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMaintenanceWindows(rows)
}

// ListMaintenanceWindowsForHost returns the windows that apply to a specific
// host: its own host-scoped windows plus every global (host_id IS NULL) one.
func (db *DB) ListMaintenanceWindowsForHost(ctx context.Context, hostID string) ([]models.MaintenanceWindow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT mw.id, mw.host_id, h.name, mw.reason, mw.starts_at, mw.ends_at, mw.created_by, mw.created_at
		FROM maintenance_windows mw
		LEFT JOIN hosts h ON h.id = mw.host_id
		WHERE mw.host_id = $1 OR mw.host_id IS NULL
		ORDER BY mw.starts_at DESC`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMaintenanceWindows(rows)
}

// GetMaintenanceWindow returns a single window by ID.
func (db *DB) GetMaintenanceWindow(ctx context.Context, id string) (*models.MaintenanceWindow, error) {
	row := db.conn.QueryRowContext(ctx, `
		SELECT mw.id, mw.host_id, h.name, mw.reason, mw.starts_at, mw.ends_at, mw.created_by, mw.created_at
		FROM maintenance_windows mw
		LEFT JOIN hosts h ON h.id = mw.host_id
		WHERE mw.id = $1`, id)
	return scanMaintenanceWindow(row)
}

// CreateMaintenanceWindow inserts a new window. w.HostID is nil for a global
// (all-hosts) window.
func (db *DB) CreateMaintenanceWindow(ctx context.Context, w models.MaintenanceWindow) (*models.MaintenanceWindow, error) {
	var id string
	err := db.conn.QueryRowContext(ctx, `
		INSERT INTO maintenance_windows (host_id, reason, starts_at, ends_at, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id`,
		w.HostID, w.Reason, w.StartsAt, w.EndsAt, w.CreatedBy,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create maintenance window: %w", err)
	}
	return db.GetMaintenanceWindow(ctx, id)
}

// DeleteMaintenanceWindow removes a window by ID.
func (db *DB) DeleteMaintenanceWindow(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM maintenance_windows WHERE id=$1`, id)
	return err
}

// IsHostInMaintenance reports whether hostID currently falls under an active
// maintenance window — its own, or a global (host_id IS NULL) one. Called by
// the alert engine once per evaluation target. A synthetic target ID (e.g.
// "docker:container:...", "proxmox:node:...") never matches a real hosts.id,
// so it naturally only ever matches a global window — same trade-off as
// resolvableHostID in internal/handlers/host_authz.go.
func (db *DB) IsHostInMaintenance(ctx context.Context, hostID string) (bool, error) {
	var active bool
	err := db.conn.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM maintenance_windows
			WHERE (host_id = $1 OR host_id IS NULL)
			  AND starts_at <= now() AND ends_at >= now()
		)`, hostID).Scan(&active)
	return active, err
}

func scanMaintenanceWindows(rows *sql.Rows) ([]models.MaintenanceWindow, error) {
	var windows []models.MaintenanceWindow
	for rows.Next() {
		var w models.MaintenanceWindow
		var hostID, hostName sql.NullString
		if err := rows.Scan(&w.ID, &hostID, &hostName, &w.Reason, &w.StartsAt, &w.EndsAt, &w.CreatedBy, &w.CreatedAt); err != nil {
			return nil, err
		}
		if hostID.Valid {
			w.HostID = &hostID.String
		}
		if hostName.Valid {
			w.HostName = &hostName.String
		}
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

func scanMaintenanceWindow(row *sql.Row) (*models.MaintenanceWindow, error) {
	var w models.MaintenanceWindow
	var hostID, hostName sql.NullString
	if err := row.Scan(&w.ID, &hostID, &hostName, &w.Reason, &w.StartsAt, &w.EndsAt, &w.CreatedBy, &w.CreatedAt); err != nil {
		return nil, err
	}
	if hostID.Valid {
		w.HostID = &hostID.String
	}
	if hostName.Valid {
		w.HostName = &hostName.String
	}
	return &w, nil
}
