package database

import (
	"time"

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

// GetLatestProxmoxGuestMetricPercent returns the freshest guest CPU% and RAM% sample.
// cpu_usage is stored as 0-1 ratio in DB and is converted to 0-100 percentage.
func (db *DB) GetLatestProxmoxGuestMetricPercent(guestID string) (cpuPercent float64, memoryPercent float64, ts time.Time, err error) {
	err = db.conn.QueryRow(`
		SELECT
			cpu_usage * 100,
			CASE WHEN mem_total > 0 THEN mem_used::float / mem_total * 100 ELSE 0 END,
			timestamp
		FROM proxmox_guest_metrics
		WHERE guest_id = $1
		ORDER BY timestamp DESC
		LIMIT 1`,
		guestID,
	).Scan(&cpuPercent, &memoryPercent, &ts)
	return cpuPercent, memoryPercent, ts, err
}

// GetMaxProxmoxGuestCPUUsagePercent returns the maximum freshest guest CPU usage across all guests.
func (db *DB) GetMaxProxmoxGuestCPUUsagePercent() float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`gm.cpu_usage * 100`)
}

// GetMaxProxmoxGuestCPUUsagePercentByConnection returns the maximum freshest guest CPU usage for one connection.
func (db *DB) GetMaxProxmoxGuestCPUUsagePercentByConnection(connectionID string) float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`gm.cpu_usage * 100`, `WHERE g.connection_id = $1`, connectionID)
}

// GetMaxProxmoxGuestCPUUsagePercentByNode returns the maximum freshest guest CPU usage for one node.
func (db *DB) GetMaxProxmoxGuestCPUUsagePercentByNode(nodeID string) float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`gm.cpu_usage * 100`, `JOIN proxmox_nodes n ON n.connection_id = g.connection_id AND n.node_name = g.node_name WHERE n.id = $1`, nodeID)
}

// GetMaxProxmoxGuestMemoryUsagePercent returns the maximum freshest guest memory usage across all guests.
func (db *DB) GetMaxProxmoxGuestMemoryUsagePercent() float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`CASE WHEN gm.mem_total > 0 THEN gm.mem_used::float / gm.mem_total * 100 ELSE 0 END`)
}

// GetMaxProxmoxGuestMemoryUsagePercentByConnection returns the maximum freshest guest memory usage for one connection.
func (db *DB) GetMaxProxmoxGuestMemoryUsagePercentByConnection(connectionID string) float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`CASE WHEN gm.mem_total > 0 THEN gm.mem_used::float / gm.mem_total * 100 ELSE 0 END`, `WHERE g.connection_id = $1`, connectionID)
}

// GetMaxProxmoxGuestMemoryUsagePercentByNode returns the maximum freshest guest memory usage for one node.
func (db *DB) GetMaxProxmoxGuestMemoryUsagePercentByNode(nodeID string) float64 {
	return db.getMaxLatestProxmoxGuestMetricPercent(`CASE WHEN gm.mem_total > 0 THEN gm.mem_used::float / gm.mem_total * 100 ELSE 0 END`, `JOIN proxmox_nodes n ON n.connection_id = g.connection_id AND n.node_name = g.node_name WHERE n.id = $1`, nodeID)
}

func (db *DB) getMaxLatestProxmoxGuestMetricPercent(metricExpr string, extraClausesAndArgs ...interface{}) float64 {
	whereClause := ""
	args := []interface{}{}
	if len(extraClausesAndArgs) > 0 {
		if clause, ok := extraClausesAndArgs[0].(string); ok {
			whereClause = clause
			args = append(args, extraClausesAndArgs[1:]...)
		}
	}

	query := `
		SELECT COALESCE(MAX(latest.metric_value), 0)
		FROM (
			SELECT DISTINCT ON (gm.guest_id)
				gm.guest_id,
				` + metricExpr + ` AS metric_value
			FROM proxmox_guest_metrics gm
			JOIN proxmox_guests g ON g.id = gm.guest_id
			` + whereClause + `
			ORDER BY gm.guest_id, gm.timestamp DESC
		) latest`

	var pct float64
	if len(args) == 0 {
		_ = db.conn.QueryRow(query).Scan(&pct)
	} else {
		_ = db.conn.QueryRow(query, args...).Scan(&pct)
	}
	return pct
}

// GetMaxProxmoxNodePendingUpdates returns the maximum pending update count across online nodes.
func (db *DB) GetMaxProxmoxNodePendingUpdates() int {
	return db.queryMaxProxmoxNodeInt(`TRUE`, nil)
}

// GetMaxProxmoxNodePendingUpdatesByConnection returns the maximum pending update count for a connection.
func (db *DB) GetMaxProxmoxNodePendingUpdatesByConnection(connectionID string) int {
	return db.queryMaxProxmoxNodeInt(`connection_id = $1`, []interface{}{connectionID})
}

// GetMaxProxmoxNodePendingUpdatesByNode returns the pending update count for one node.
func (db *DB) GetMaxProxmoxNodePendingUpdatesByNode(nodeID string) int {
	return db.queryMaxProxmoxNodeInt(`id = $1`, []interface{}{nodeID})
}

// GetMaxProxmoxNodeSecurityUpdates returns the maximum security update count across online nodes.
func (db *DB) GetMaxProxmoxNodeSecurityUpdates() int {
	return db.queryMaxProxmoxNodeSecurityInt(`TRUE`, nil)
}

// GetMaxProxmoxNodeSecurityUpdatesByConnection returns the maximum security update count for a connection.
func (db *DB) GetMaxProxmoxNodeSecurityUpdatesByConnection(connectionID string) int {
	return db.queryMaxProxmoxNodeSecurityInt(`connection_id = $1`, []interface{}{connectionID})
}

// GetMaxProxmoxNodeSecurityUpdatesByNode returns the security update count for one node.
func (db *DB) GetMaxProxmoxNodeSecurityUpdatesByNode(nodeID string) int {
	return db.queryMaxProxmoxNodeSecurityInt(`id = $1`, []interface{}{nodeID})
}

func (db *DB) queryMaxProxmoxNodeInt(whereClause string, args []interface{}) int {
	query := `SELECT COALESCE(MAX(pending_updates), 0) FROM proxmox_nodes WHERE ` + whereClause
	var value int
	if len(args) == 0 {
		_ = db.conn.QueryRow(query).Scan(&value)
	} else {
		_ = db.conn.QueryRow(query, args...).Scan(&value)
	}
	return value
}

func (db *DB) queryMaxProxmoxNodeSecurityInt(whereClause string, args []interface{}) int {
	query := `SELECT COALESCE(MAX(security_updates), 0) FROM proxmox_nodes WHERE ` + whereClause
	var value int
	if len(args) == 0 {
		_ = db.conn.QueryRow(query).Scan(&value)
	} else {
		_ = db.conn.QueryRow(query, args...).Scan(&value)
	}
	return value
}

// GetRecentFailedTaskCountByConnection returns the number of failed tasks since the given time for one connection.
func (db *DB) GetRecentFailedTaskCountByConnection(connectionID string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM proxmox_tasks
		WHERE connection_id = $1
		  AND status='stopped' AND exit_status != '' AND exit_status != 'OK'
		  AND start_time >= $2`,
		connectionID, since).Scan(&count)
	return count, err
}

// GetRecentFailedTaskCountByNode returns the number of failed tasks since the given time for one node.
func (db *DB) GetRecentFailedTaskCountByNode(connectionID, nodeName string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM proxmox_tasks
		WHERE connection_id = $1 AND node_name = $2
		  AND status='stopped' AND exit_status != '' AND exit_status != 'OK'
		  AND start_time >= $3`,
		connectionID, nodeName, since).Scan(&count)
	return count, err
}

// GetRecentFailedTaskCountByNodeID returns failed task count for a Proxmox node UUID.
func (db *DB) GetRecentFailedTaskCountByNodeID(nodeID string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM proxmox_tasks t
		JOIN proxmox_nodes n ON n.connection_id = t.connection_id AND n.node_name = t.node_name
		WHERE n.id = $1
		  AND t.status='stopped' AND t.exit_status != '' AND t.exit_status != 'OK'
		  AND t.start_time >= $2`,
		nodeID, since).Scan(&count)
	return count, err
}

// GetProxmoxDiskFailedCount returns the number of failed disks across all active disks.
func (db *DB) GetProxmoxDiskFailedCount() int {
	return db.queryProxmoxDiskCount(`health = 'FAILED'`, nil)
}

// GetProxmoxDiskFailedCountByConnection returns the number of failed disks for a connection.
func (db *DB) GetProxmoxDiskFailedCountByConnection(connectionID string) int {
	return db.queryProxmoxDiskCount(`connection_id = $1 AND health = 'FAILED'`, []interface{}{connectionID})
}

// GetProxmoxDiskFailedCountByNode returns the number of failed disks for a node.
func (db *DB) GetProxmoxDiskFailedCountByNode(connectionID, nodeName string) int {
	return db.queryProxmoxDiskCount(`connection_id = $1 AND node_name = $2 AND health = 'FAILED'`, []interface{}{connectionID, nodeName})
}

// GetProxmoxDiskFailedCountByNodeID returns the number of failed disks for a node UUID.
func (db *DB) GetProxmoxDiskFailedCountByNodeID(nodeID string) int {
	return db.queryProxmoxDiskCount(`EXISTS (
		SELECT 1 FROM proxmox_nodes n
		WHERE n.id = $1
		  AND n.connection_id = proxmox_disks.connection_id
		  AND n.node_name = proxmox_disks.node_name
	) AND health = 'FAILED'`, []interface{}{nodeID})
}

func (db *DB) queryProxmoxDiskCount(whereClause string, args []interface{}) int {
	query := `SELECT COALESCE(COUNT(*), 0) FROM proxmox_disks WHERE ` + whereClause
	var count int
	if len(args) == 0 {
		_ = db.conn.QueryRow(query).Scan(&count)
	} else {
		_ = db.conn.QueryRow(query, args...).Scan(&count)
	}
	return count
}

// GetProxmoxDiskMinWearoutPercent returns the minimum wearout percentage across active disks.
func (db *DB) GetProxmoxDiskMinWearoutPercent() float64 {
	return db.queryProxmoxDiskMinWearout(`wearout >= 0`, nil)
}

// GetProxmoxDiskMinWearoutPercentByConnection returns the minimum wearout percentage for a connection.
func (db *DB) GetProxmoxDiskMinWearoutPercentByConnection(connectionID string) float64 {
	return db.queryProxmoxDiskMinWearout(`connection_id = $1 AND wearout >= 0`, []interface{}{connectionID})
}

// GetProxmoxDiskMinWearoutPercentByNode returns the minimum wearout percentage for a node.
func (db *DB) GetProxmoxDiskMinWearoutPercentByNode(connectionID, nodeName string) float64 {
	return db.queryProxmoxDiskMinWearout(`connection_id = $1 AND node_name = $2 AND wearout >= 0`, []interface{}{connectionID, nodeName})
}

// GetProxmoxDiskMinWearoutPercentByNodeID returns the minimum wearout percentage for a node UUID.
func (db *DB) GetProxmoxDiskMinWearoutPercentByNodeID(nodeID string) float64 {
	return db.queryProxmoxDiskMinWearout(`EXISTS (
		SELECT 1 FROM proxmox_nodes n
		WHERE n.id = $1
		  AND n.connection_id = proxmox_disks.connection_id
		  AND n.node_name = proxmox_disks.node_name
	) AND wearout >= 0`, []interface{}{nodeID})
}

func (db *DB) queryProxmoxDiskMinWearout(whereClause string, args []interface{}) float64 {
	query := `SELECT COALESCE(MIN(wearout), 0) FROM proxmox_disks WHERE ` + whereClause
	var pct float64
	if len(args) == 0 {
		_ = db.conn.QueryRow(query).Scan(&pct)
	} else {
		_ = db.conn.QueryRow(query, args...).Scan(&pct)
	}
	return pct
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
