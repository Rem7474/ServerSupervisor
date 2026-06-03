// Package database — Proxmox aggregate queries: summary tiles and the maximum
// usage / sensor metrics used by the dashboard + alert engine. CRUD operations
// live in db_proxmox_connections.go, db_proxmox_nodes.go and
// db_proxmox_guests.go.
package database

import (
	"context"

	"github.com/serversupervisor/server/internal/models"
)

// GetProxmoxSummary returns aggregate stats and health signals across all connections.
func (db *DB) GetProxmoxSummary(ctx context.Context) (models.ProxmoxSummary, error) {
	var s models.ProxmoxSummary

	queries := []struct {
		dest *int
		q    string
	}{
		{&s.ConnectionCount, `SELECT COUNT(*) FROM proxmox_connections`},
		{&s.NodeCount, `SELECT COUNT(*) FROM proxmox_nodes`},
		{&s.VMCount, `SELECT COUNT(*) FROM proxmox_guests WHERE guest_type='vm'`},
		{&s.LXCCount, `SELECT COUNT(*) FROM proxmox_guests WHERE guest_type='lxc'`},
		{&s.NodesDown, `SELECT COUNT(*) FROM proxmox_nodes WHERE status != 'online'`},
		{&s.StorageNearFull, `SELECT COUNT(*) FROM proxmox_storages WHERE total > 0 AND (used::float / total::float) > 0.80`},
		{&s.StorageOffline, `SELECT COUNT(*) FROM proxmox_storages WHERE active = FALSE OR enabled = FALSE`},
		{&s.RecentFailedTasks, `SELECT COUNT(*) FROM proxmox_tasks WHERE status='stopped' AND exit_status != '' AND exit_status != 'OK' AND start_time >= NOW() - INTERVAL '24 hours'`},
	}
	for _, q := range queries {
		if err := db.conn.QueryRowContext(ctx, q.q).Scan(q.dest); err != nil {
			return s, err
		}
	}

	err := db.conn.QueryRowContext(ctx, `SELECT COALESCE(SUM(total),0), COALESCE(SUM(used),0) FROM proxmox_storages`).
		Scan(&s.StorageTotal, &s.StorageUsed)
	return s, err
}

// GetMaxProxmoxStorageUsagePercent returns the max used/total ratio (0-100) across all active Proxmox storages.
// Returns 0 if no storage data is available.
func (db *DB) GetMaxProxmoxStorageUsagePercent(ctx context.Context) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(used::float / NULLIF(total,0) * 100), 0)
		FROM proxmox_storages
		WHERE total > 0 AND enabled = TRUE AND active = TRUE
	`).Scan(&pct)
	return pct
}

// GetMaxProxmoxStorageUsagePercentByConnection returns the max used/total ratio (0-100)
// for active storages of one Proxmox connection.
func (db *DB) GetMaxProxmoxStorageUsagePercentByConnection(ctx context.Context, connectionID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(s.used::float / NULLIF(s.total,0) * 100), 0)
		FROM proxmox_storages s
		WHERE s.connection_id = $1
		  AND s.total > 0
		  AND s.enabled = TRUE
		  AND s.active = TRUE
	`, connectionID).Scan(&pct)
	return pct
}

// GetMaxProxmoxNodeCPUUsagePercent returns the maximum node CPU usage (0-100)
// across all online Proxmox nodes globally.
func (db *DB) GetMaxProxmoxNodeCPUUsagePercent(ctx context.Context) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(n.cpu_usage * 100), 0)
		FROM proxmox_nodes n
		WHERE n.status = 'online'
	`).Scan(&pct)
	return pct
}

// GetMaxProxmoxNodeCPUUsagePercentByConnection returns the maximum node CPU usage (0-100)
// for one Proxmox connection.
func (db *DB) GetMaxProxmoxNodeCPUUsagePercentByConnection(ctx context.Context, connectionID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(n.cpu_usage * 100), 0)
		FROM proxmox_nodes n
		WHERE n.connection_id = $1
		  AND n.status = 'online'
	`, connectionID).Scan(&pct)
	return pct
}

// GetProxmoxNodeCPUUsagePercentByNode returns the CPU usage (0-100) for one specific node.
func (db *DB) GetProxmoxNodeCPUUsagePercentByNode(ctx context.Context, nodeID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(n.cpu_usage * 100, 0)
		FROM proxmox_nodes n
		WHERE n.id = $1
		  AND n.status = 'online'
	`, nodeID).Scan(&pct)
	return pct
}

// GetMaxProxmoxNodeMemoryUsagePercent returns the maximum node memory usage (0-100)
// across all online Proxmox nodes globally.
func (db *DB) GetMaxProxmoxNodeMemoryUsagePercent(ctx context.Context) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CASE WHEN n.mem_total > 0
			THEN n.mem_used::float / n.mem_total * 100
			ELSE 0 END), 0)
		FROM proxmox_nodes n
		WHERE n.status = 'online'
	`).Scan(&pct)
	return pct
}

// GetMaxProxmoxNodeMemoryUsagePercentByConnection returns the maximum node memory usage (0-100)
// for one Proxmox connection.
func (db *DB) GetMaxProxmoxNodeMemoryUsagePercentByConnection(ctx context.Context, connectionID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(CASE WHEN n.mem_total > 0
			THEN n.mem_used::float / n.mem_total * 100
			ELSE 0 END), 0)
		FROM proxmox_nodes n
		WHERE n.connection_id = $1
		  AND n.status = 'online'
	`, connectionID).Scan(&pct)
	return pct
}

// GetProxmoxNodeMemoryUsagePercentByNode returns the memory usage (0-100) for one specific node.
func (db *DB) GetProxmoxNodeMemoryUsagePercentByNode(ctx context.Context, nodeID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(CASE WHEN n.mem_total > 0
			THEN n.mem_used::float / n.mem_total * 100
			ELSE 0 END, 0)
		FROM proxmox_nodes n
		WHERE n.id = $1
		  AND n.status = 'online'
	`, nodeID).Scan(&pct)
	return pct
}

// GetMaxProxmoxNodeCPUTemperature returns the maximum latest CPU temperature
// across Proxmox nodes that have a mapped source host.
func (db *DB) GetMaxProxmoxNodeCPUTemperature(ctx context.Context) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "cpu_temp_source_host_id", "cpu_temperature", `TRUE`, nil)
}

// GetMaxProxmoxNodeCPUTemperatureByConnection returns the maximum latest CPU
// temperature for nodes in one Proxmox connection.
func (db *DB) GetMaxProxmoxNodeCPUTemperatureByConnection(ctx context.Context, connectionID string) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "cpu_temp_source_host_id", "cpu_temperature", `n.connection_id = $1`, []interface{}{connectionID})
}

// GetProxmoxNodeCPUTemperatureByNode returns the latest CPU temperature for one node.
func (db *DB) GetProxmoxNodeCPUTemperatureByNode(ctx context.Context, nodeID string) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "cpu_temp_source_host_id", "cpu_temperature", `n.id = $1`, []interface{}{nodeID})
}

// GetMaxProxmoxNodeFanRPM returns the maximum latest fan RPM across Proxmox nodes
// that have a mapped source host.
func (db *DB) GetMaxProxmoxNodeFanRPM(ctx context.Context) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "fan_rpm_source_host_id", "fan_rpm", `TRUE`, nil)
}

// GetMaxProxmoxNodeFanRPMByConnection returns the maximum latest fan RPM for
// nodes in one Proxmox connection.
func (db *DB) GetMaxProxmoxNodeFanRPMByConnection(ctx context.Context, connectionID string) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "fan_rpm_source_host_id", "fan_rpm", `n.connection_id = $1`, []interface{}{connectionID})
}

// GetProxmoxNodeFanRPMByNode returns the latest fan RPM for one node.
func (db *DB) GetProxmoxNodeFanRPMByNode(ctx context.Context, nodeID string) float64 {
	return db.queryProxmoxNodeSensorMetric(ctx, "fan_rpm_source_host_id", "fan_rpm", `n.id = $1`, []interface{}{nodeID})
}

// queryProxmoxNodeSensorMetric is a shared helper that joins the latest
// system_metrics row for the host referenced by the given sourceColumn on a
// proxmox_nodes row, then returns the MAX of metricColumn for rows matching
// whereClause. Used to compute aggregate sensor metrics for the dashboard.
func (db *DB) queryProxmoxNodeSensorMetric(ctx context.Context, sourceColumn, metricColumn, whereClause string, args []interface{}) float64 {
	// Freshness guard (10 min) mirrors GetEffectiveHostCPUTemperature: a source
	// host that stopped reporting sensor data must NOT keep feeding a frozen value
	// to the dashboard or the alert engine (which would otherwise leave a
	// temperature/fan alert active indefinitely).
	query := `
		SELECT COALESCE(MAX(latest.metric_value), 0)
		FROM proxmox_nodes n
		LEFT JOIN LATERAL (
			SELECT sm.` + metricColumn + ` AS metric_value
			FROM system_metrics sm
			WHERE sm.host_id = n.` + sourceColumn + `
			  AND sm.timestamp > NOW() - INTERVAL '10 minutes'
			ORDER BY sm.timestamp DESC
			LIMIT 1
		) latest ON TRUE
		WHERE ` + whereClause + `
		  AND n.` + sourceColumn + ` IS NOT NULL
		  AND n.status = 'online'`

	var value float64
	if len(args) == 0 {
		_ = db.conn.QueryRowContext(ctx, query).Scan(&value)
	} else {
		_ = db.conn.QueryRowContext(ctx, query, args...).Scan(&value)
	}
	return value
}

// GetMaxProxmoxStorageUsagePercentByNode returns the max used/total ratio (0-100)
// for active storages on one Proxmox node identified by proxmox_nodes.id.
func (db *DB) GetMaxProxmoxStorageUsagePercentByNode(ctx context.Context, nodeID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(s.used::float / NULLIF(s.total,0) * 100), 0)
		FROM proxmox_storages s
		JOIN proxmox_nodes n
		  ON n.connection_id = s.connection_id AND n.node_name = s.node_name
		WHERE n.id = $1
		  AND s.total > 0
		  AND s.enabled = TRUE
		  AND s.active = TRUE
	`, nodeID).Scan(&pct)
	return pct
}

// GetProxmoxStorageUsagePercentByStorage returns used/total ratio (0-100)
// for one storage identified by proxmox_storages.id.
func (db *DB) GetProxmoxStorageUsagePercentByStorage(ctx context.Context, storageID string) float64 {
	var pct float64
	_ = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(s.used::float / NULLIF(s.total,0) * 100, 0)
		FROM proxmox_storages s
		WHERE s.id = $1
		  AND s.total > 0
		  AND s.enabled = TRUE
		  AND s.active = TRUE
	`, storageID).Scan(&pct)
	return pct
}
