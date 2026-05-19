package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Disk Metrics ==========

func (db *DB) InsertDiskMetrics(ctx context.Context, metrics []models.DiskMetrics) error {
	if len(metrics) == 0 {
		return nil
	}
	for _, m := range metrics {
		_, err := db.conn.ExecContext(ctx, 
			`INSERT INTO disk_metrics (
				host_id, timestamp, mount_point, filesystem,
				size_gb, used_gb, avail_gb, used_percent,
				inodes_total, inodes_used, inodes_free, inodes_percent
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			m.HostID, m.Timestamp, m.MountPoint, m.Filesystem,
			m.SizeGB, m.UsedGB, m.AvailGB, m.UsedPercent,
			m.InodesTotal, m.InodesUsed, m.InodesFree, m.InodesPercent,
		)
		if err != nil {
			return fmt.Errorf("failed to insert disk metrics: %w", err)
		}
	}
	return nil
}

func (db *DB) GetLatestDiskMetrics(ctx context.Context, hostID string) ([]models.DiskMetrics, error) {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT id, host_id, timestamp, mount_point, filesystem,
			size_gb, used_gb, avail_gb, used_percent,
			inodes_total, inodes_used, inodes_free, inodes_percent
		FROM disk_metrics
		WHERE host_id = $1
		  AND timestamp = (
			SELECT MAX(timestamp)
			FROM disk_metrics
			WHERE host_id = $1
		  )
		ORDER BY mount_point ASC`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.DiskMetrics
	for rows.Next() {
		var m models.DiskMetrics
		if err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.MountPoint, &m.Filesystem,
			&m.SizeGB, &m.UsedGB, &m.AvailGB, &m.UsedPercent,
			&m.InodesTotal, &m.InodesUsed, &m.InodesFree, &m.InodesPercent,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (db *DB) GetDiskMetricsHistory(ctx context.Context, hostID, mountPoint string, limit int) ([]models.DiskMetrics, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT id, host_id, timestamp, mount_point, filesystem,
			   size_gb, used_gb, avail_gb, used_percent,
			   inodes_total, inodes_used, inodes_free, inodes_percent
		FROM disk_metrics
		WHERE host_id = $1 AND mount_point = $2
		ORDER BY timestamp DESC
		LIMIT $3`,
		hostID, mountPoint, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.DiskMetrics
	for rows.Next() {
		var m models.DiskMetrics
		if err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.MountPoint, &m.Filesystem,
			&m.SizeGB, &m.UsedGB, &m.AvailGB, &m.UsedPercent,
			&m.InodesTotal, &m.InodesUsed, &m.InodesFree, &m.InodesPercent,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (db *DB) GetDiskMetricsAggregated(ctx context.Context, hostID, mountPoint string, hours int) ([]models.DiskMetrics, string, error) {
	if hours <= 0 {
		hours = 24
	}

	var (
		rows    *sql.Rows
		err     error
		aggType string
	)

	if hours <= 24 {
		aggType = "raw"
		rows, err = db.conn.QueryContext(ctx, 
			`SELECT id, host_id, timestamp, mount_point, filesystem,
				size_gb, used_gb, avail_gb, used_percent,
				inodes_total, inodes_used, inodes_free, inodes_percent
			FROM disk_metrics
			WHERE host_id = $1 AND mount_point = $2
			  AND timestamp > NOW() - INTERVAL '1 hour' * $3
			ORDER BY timestamp ASC`,
			hostID, mountPoint, hours,
		)
	} else if hours <= 720 {
		aggType = "hour"
		rows, err = db.conn.QueryContext(ctx, 
			`SELECT
				0 AS id,
				$1 AS host_id,
				date_trunc('hour', timestamp) AS timestamp,
				mount_point,
				'' AS filesystem,
				AVG(size_gb) AS size_gb,
				AVG(used_gb) AS used_gb,
				AVG(avail_gb) AS avail_gb,
				AVG(used_percent) AS used_percent,
				0 AS inodes_total, 0 AS inodes_used, 0 AS inodes_free, 0.0 AS inodes_percent
			FROM disk_metrics
			WHERE host_id = $1 AND mount_point = $2
			  AND timestamp > NOW() - INTERVAL '1 hour' * $3
			GROUP BY date_trunc('hour', timestamp), mount_point
			ORDER BY timestamp ASC`,
			hostID, mountPoint, hours,
		)
	} else {
		aggType = "day"
		rows, err = db.conn.QueryContext(ctx, 
			`SELECT
				0 AS id,
				$1 AS host_id,
				date_trunc('day', timestamp) AS timestamp,
				mount_point,
				'' AS filesystem,
				AVG(size_gb) AS size_gb,
				AVG(used_gb) AS used_gb,
				AVG(avail_gb) AS avail_gb,
				AVG(used_percent) AS used_percent,
				0 AS inodes_total, 0 AS inodes_used, 0 AS inodes_free, 0.0 AS inodes_percent
			FROM disk_metrics
			WHERE host_id = $1 AND mount_point = $2
			  AND timestamp > NOW() - INTERVAL '1 hour' * $3
			GROUP BY date_trunc('day', timestamp), mount_point
			ORDER BY timestamp ASC`,
			hostID, mountPoint, hours,
		)
	}
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.DiskMetrics
	for rows.Next() {
		var m models.DiskMetrics
		if err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.MountPoint, &m.Filesystem,
			&m.SizeGB, &m.UsedGB, &m.AvailGB, &m.UsedPercent,
			&m.InodesTotal, &m.InodesUsed, &m.InodesFree, &m.InodesPercent,
		); err != nil {
			return nil, "", err
		}
		metrics = append(metrics, m)
	}
	return metrics, aggType, nil
}

// ========== Disk Health (SMART) ==========

func (db *DB) InsertDiskHealth(ctx context.Context, healthData []models.DiskHealth) error {
	if len(healthData) == 0 {
		return nil
	}
	for _, h := range healthData {
		_, err := db.conn.ExecContext(ctx, 
			`INSERT INTO disk_health (
				host_id, timestamp, device, model, serial_number,
				smart_status, temperature, power_on_hours, power_cycles,
				realloc_sectors, pending_sectors
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			h.HostID, h.CollectedAt, h.Device, h.Model, h.SerialNumber,
			h.SmartStatus, h.Temperature, h.PowerOnHours, h.PowerCycles,
			h.ReallocSectors, h.PendingSectors,
		)
		if err != nil {
			return fmt.Errorf("failed to insert disk health: %w", err)
		}
	}
	return nil
}

func (db *DB) GetLatestDiskHealth(ctx context.Context, hostID string) ([]models.DiskHealth, error) {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT DISTINCT ON (device)
			id, host_id, timestamp, device, model, serial_number,
			smart_status, temperature, power_on_hours, power_cycles,
			realloc_sectors, pending_sectors
		FROM disk_health
		WHERE host_id = $1
		ORDER BY device, timestamp DESC`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var healthData []models.DiskHealth
	for rows.Next() {
		var h models.DiskHealth
		if err := rows.Scan(
			&h.ID, &h.HostID, &h.CollectedAt, &h.Device, &h.Model, &h.SerialNumber,
			&h.SmartStatus, &h.Temperature, &h.PowerOnHours, &h.PowerCycles,
			&h.ReallocSectors, &h.PendingSectors,
		); err != nil {
			return nil, err
		}
		healthData = append(healthData, h)
	}
	return healthData, nil
}
