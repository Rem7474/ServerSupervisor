package database

import (
	"github.com/serversupervisor/server/internal/models"
)

// InsertProxmoxNodeMetric stores a point-in-time snapshot of a node's CPU/RAM.
// Called by the poller after each successful UpsertProxmoxNode.
func (db *DB) InsertProxmoxNodeMetric(nodeID, connectionID, nodeName string, cpuUsage float64, memTotal, memUsed int64) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_node_metrics (node_id, connection_id, node_name, cpu_usage, mem_total, mem_used, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())`,
		nodeID, connectionID, nodeName, cpuUsage, memTotal, memUsed,
	)
	return err
}

// GetProxmoxNodeMetricsSummary returns time-bucketed avg CPU% and RAM% across all nodes,
// using the same bucket logic as GetMetricsSummary for host agents.
func (db *DB) GetProxmoxNodeMetricsSummary(hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	var (
		query string
		args  []interface{}
	)

	// cpu_usage is stored as a 0‒1 ratio; multiply by 100 to get percentage.
	// mem_avg is computed as (mem_used / mem_total) * 100 when mem_total > 0.
	bucketExpr := ""
	if db.hasTimescaleDB {
		bucketExpr = `time_bucket($2 * '1 minute'::interval, timestamp)`
	} else {
		bucketExpr = `to_timestamp(floor(EXTRACT(EPOCH FROM timestamp) / ($2 * 60)) * ($2 * 60))`
	}

	query = `
		SELECT
			` + bucketExpr + ` AS ts,
			AVG(cpu_usage * 100) AS cpu_avg,
			AVG(CASE WHEN mem_total > 0 THEN mem_used::float / mem_total * 100 ELSE 0 END) AS mem_avg,
			COUNT(*) AS sample_count
		FROM proxmox_node_metrics
		WHERE timestamp > NOW() - INTERVAL '1 hour' * $1
		GROUP BY ts
		ORDER BY ts ASC`
	args = []interface{}{hours, bucketMinutes}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var summary []models.ProxmoxNodeMetricsSummary
	for rows.Next() {
		var s models.ProxmoxNodeMetricsSummary
		if err := rows.Scan(&s.Timestamp, &s.CPUAvg, &s.MemoryAvg, &s.SampleCount); err != nil {
			continue
		}
		summary = append(summary, s)
	}
	return summary, rows.Err()
}

// CleanOldProxmoxNodeMetrics removes snapshots older than retentionDays.
// Mirrors CleanOldMetrics for host agent data.
func (db *DB) CleanOldProxmoxNodeMetrics(retentionDays int) (int64, error) {
	res, err := db.conn.Exec(
		`DELETE FROM proxmox_node_metrics WHERE timestamp < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ─── Guest metrics ────────────────────────────────────────────────────────────

// InsertProxmoxGuestMetric stores a point-in-time snapshot of a guest's CPU/RAM.
// Called by the poller after each successful UpsertProxmoxGuest for running guests.
func (db *DB) InsertProxmoxGuestMetric(guestID string, cpuUsage float64, memTotal, memUsed int64) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_guest_metrics (guest_id, cpu_usage, mem_total, mem_used, timestamp)
		VALUES ($1, $2, $3, $4, NOW())`,
		guestID, cpuUsage, memTotal, memUsed,
	)
	return err
}

// GetProxmoxGuestMetricsSummary returns time-bucketed CPU% and RAM% for a single guest.
// Same format as ProxmoxNodeMetricsSummary so the frontend can reuse the same chart logic.
func (db *DB) GetProxmoxGuestMetricsSummary(guestID string, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	bucketExpr := ""
	if db.hasTimescaleDB {
		bucketExpr = `time_bucket($3 * '1 minute'::interval, timestamp)`
	} else {
		bucketExpr = `to_timestamp(floor(EXTRACT(EPOCH FROM timestamp) / ($3 * 60)) * ($3 * 60))`
	}

	query := `
		SELECT
			` + bucketExpr + ` AS ts,
			AVG(cpu_usage * 100) AS cpu_avg,
			AVG(CASE WHEN mem_total > 0 THEN mem_used::float / mem_total * 100 ELSE 0 END) AS mem_avg,
			COUNT(*) AS sample_count
		FROM proxmox_guest_metrics
		WHERE guest_id = $1
		  AND timestamp > NOW() - INTERVAL '1 hour' * $2
		GROUP BY ts
		ORDER BY ts ASC`

	rows, err := db.conn.Query(query, guestID, hours, bucketMinutes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var summary []models.ProxmoxNodeMetricsSummary
	for rows.Next() {
		var s models.ProxmoxNodeMetricsSummary
		if err := rows.Scan(&s.Timestamp, &s.CPUAvg, &s.MemoryAvg, &s.SampleCount); err != nil {
			continue
		}
		summary = append(summary, s)
	}
	return summary, rows.Err()
}

// CleanOldProxmoxGuestMetrics removes guest snapshots older than retentionDays.
func (db *DB) CleanOldProxmoxGuestMetrics(retentionDays int) (int64, error) {
	res, err := db.conn.Exec(
		`DELETE FROM proxmox_guest_metrics WHERE timestamp < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetNodeIDByConnectionAndName resolves a proxmox_nodes UUID from its composite key.
// Used by the poller so we can insert node metrics without an extra JOIN.
func (db *DB) GetProxmoxNodeID(connectionID, nodeName string) (string, error) {
	var id string
	err := db.conn.QueryRow(
		`SELECT id FROM proxmox_nodes WHERE connection_id = $1 AND node_name = $2`,
		connectionID, nodeName,
	).Scan(&id)
	return id, err
}

// GetProxmoxNodeMetricsSummaryByNode returns time-bucketed stats for a single node.
func (db *DB) GetProxmoxNodeMetricsSummaryByNode(nodeID string, hours, bucketMinutes int) ([]models.ProxmoxNodeMetricsSummary, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	bucketExpr := ""
	if db.hasTimescaleDB {
		bucketExpr = `time_bucket($3 * '1 minute'::interval, timestamp)`
	} else {
		bucketExpr = `to_timestamp(floor(EXTRACT(EPOCH FROM timestamp) / ($3 * 60)) * ($3 * 60))`
	}

	query := `
		SELECT
			` + bucketExpr + ` AS ts,
			AVG(cpu_usage * 100) AS cpu_avg,
			AVG(CASE WHEN mem_total > 0 THEN mem_used::float / mem_total * 100 ELSE 0 END) AS mem_avg,
			COUNT(*) AS sample_count
		FROM proxmox_node_metrics
		WHERE node_id = $1
		  AND timestamp > NOW() - INTERVAL '1 hour' * $2
		GROUP BY ts
		ORDER BY ts ASC`

	rows, err := db.conn.Query(query, nodeID, hours, bucketMinutes)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var summary []models.ProxmoxNodeMetricsSummary
	for rows.Next() {
		var s models.ProxmoxNodeMetricsSummary
		if err := rows.Scan(&s.Timestamp, &s.CPUAvg, &s.MemoryAvg, &s.SampleCount); err != nil {
			continue
		}
		summary = append(summary, s)
	}
	return summary, rows.Err()
}
