package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// UpsertProxmoxNode inserts or updates a node record.
func (db *DB) UpsertProxmoxNode(ctx context.Context, connectionID, nodeName, status string, cpuCount int, cpuUsage float64, memTotal, memUsed, uptime int64, pveVersion, clusterName, ipAddress string) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO proxmox_nodes
		    (connection_id, node_name, status, cpu_count, cpu_usage, mem_total, mem_used,
		     uptime, pve_version, cluster_name, ip_address, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NOW())
		ON CONFLICT (connection_id, node_name) DO UPDATE SET
		    status        = EXCLUDED.status,
		    cpu_count     = EXCLUDED.cpu_count,
		    cpu_usage     = EXCLUDED.cpu_usage,
		    mem_total     = EXCLUDED.mem_total,
		    mem_used      = EXCLUDED.mem_used,
		    uptime        = EXCLUDED.uptime,
		    pve_version   = EXCLUDED.pve_version,
		    cluster_name  = EXCLUDED.cluster_name,
		    ip_address    = EXCLUDED.ip_address,
		    last_seen_at  = NOW()`,
		connectionID, nodeName, status, cpuCount, cpuUsage, memTotal, memUsed,
		uptime, pveVersion, clusterName, ipAddress,
	)
	return err
}

// nodeSelectCols is the shared SELECT list used by list + get queries.
const nodeSelectCols = `
	n.id, n.connection_id, n.node_name, n.status,
	COALESCE(n.cpu_temp_source_host_id, '') AS cpu_temp_source_host_id,
	MAX(COALESCE(src.hostname, src.name, '')) AS cpu_temp_source_host_name,
	COALESCE(n.fan_rpm_source_host_id, '') AS fan_rpm_source_host_id,
	MAX(COALESCE(src_fan.hostname, src_fan.name, '')) AS fan_rpm_source_host_name,
	n.cpu_count, n.cpu_usage, n.mem_total, n.mem_used,
	n.uptime, n.pve_version, n.cluster_name, n.ip_address, n.last_seen_at,
	n.pending_updates, n.security_updates, n.last_update_check_at,
	COUNT(CASE WHEN g.guest_type='vm'  THEN 1 END) AS vm_count,
	COUNT(CASE WHEN g.guest_type='lxc' THEN 1 END) AS lxc_count`

// ListProxmoxNodes returns all nodes with VM/LXC counts.
func (db *DB) ListProxmoxNodes(ctx context.Context) ([]models.ProxmoxNode, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT `+nodeSelectCols+`
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
		LEFT JOIN hosts src_fan     ON src_fan.id = n.fan_rpm_source_host_id
		LEFT JOIN proxmox_guests g ON g.connection_id=n.connection_id AND g.node_name=n.node_name
		GROUP BY n.id
		ORDER BY n.cluster_name, n.node_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNodes(rows)
}

// ListProxmoxNodesByConnection returns nodes for one connection.
func (db *DB) ListProxmoxNodesByConnection(ctx context.Context, connectionID string) ([]models.ProxmoxNode, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT `+nodeSelectCols+`
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
		LEFT JOIN hosts src_fan     ON src_fan.id = n.fan_rpm_source_host_id
		LEFT JOIN proxmox_guests g ON g.connection_id=n.connection_id AND g.node_name=n.node_name
		WHERE n.connection_id=$1
		GROUP BY n.id
		ORDER BY n.node_name`, connectionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNodes(rows)
}

// GetProxmoxNode returns a node by ID including its guests, storages, disks and recent tasks.
func (db *DB) GetProxmoxNode(ctx context.Context, id string) (*models.ProxmoxNode, error) {
	var n models.ProxmoxNode
	var lastUpdateCheckAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT `+nodeSelectCols+`
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
		LEFT JOIN hosts src_fan     ON src_fan.id = n.fan_rpm_source_host_id
		LEFT JOIN proxmox_guests g ON g.connection_id=n.connection_id AND g.node_name=n.node_name
		WHERE n.id=$1
		GROUP BY n.id`, id).Scan(
		&n.ID, &n.ConnectionID, &n.NodeName, &n.Status,
		&n.CPUTempSourceHostID, &n.CPUTempSourceHostName,
		&n.FanRPMSourceHostID, &n.FanRPMSourceHostName,
		&n.CPUCount, &n.CPUUsage, &n.MemTotal, &n.MemUsed,
		&n.Uptime, &n.PVEVersion, &n.ClusterName, &n.IPAddress, &n.LastSeenAt,
		&n.PendingUpdates, &n.SecurityUpdates, &lastUpdateCheckAt,
		&n.VMCount, &n.LXCCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastUpdateCheckAt.Valid {
		t := lastUpdateCheckAt.Time
		n.LastUpdateCheckAt = &t
	}

	// Load guests
	guests, err := db.ListProxmoxGuestsByNode(ctx, n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Guests = guests

	// Load storages
	storages, err := db.ListProxmoxStoragesByNode(ctx, n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Storages = storages

	// Load physical disks
	disks, err := db.ListProxmoxDisksByNode(ctx, n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Disks = disks

	// Load 50 most recent tasks
	tasks, err := db.ListProxmoxTasksByNode(ctx, n.ConnectionID, n.NodeName, 50)
	if err != nil {
		return nil, err
	}
	n.Tasks = tasks

	return &n, nil
}

func scanNodes(rows *sql.Rows) ([]models.ProxmoxNode, error) {
	var nodes []models.ProxmoxNode
	for rows.Next() {
		var n models.ProxmoxNode
		var lastUpdateCheckAt sql.NullTime
		if err := rows.Scan(
			&n.ID, &n.ConnectionID, &n.NodeName, &n.Status,
			&n.CPUTempSourceHostID, &n.CPUTempSourceHostName,
			&n.FanRPMSourceHostID, &n.FanRPMSourceHostName,
			&n.CPUCount, &n.CPUUsage, &n.MemTotal, &n.MemUsed,
			&n.Uptime, &n.PVEVersion, &n.ClusterName, &n.IPAddress, &n.LastSeenAt,
			&n.PendingUpdates, &n.SecurityUpdates, &lastUpdateCheckAt,
			&n.VMCount, &n.LXCCount,
		); err != nil {
			return nil, err
		}
		if lastUpdateCheckAt.Valid {
			t := lastUpdateCheckAt.Time
			n.LastUpdateCheckAt = &t
		}
		nodes = append(nodes, n)
	}
	if nodes == nil {
		nodes = []models.ProxmoxNode{}
	}
	return nodes, rows.Err()
}

// SetProxmoxNodeSensorSource maps both CPU temperature and fan RPM sources to the same host.
// Pass an empty hostID to clear both mappings.
func (db *DB) SetProxmoxNodeSensorSource(ctx context.Context, nodeID, hostID string) error {
	if hostID == "" {
		_, err := db.conn.ExecContext(ctx, `
			UPDATE proxmox_nodes
			SET cpu_temp_source_host_id = NULL,
			    fan_rpm_source_host_id = NULL
			WHERE id = $1`, nodeID)
		return err
	}

	_, err := db.conn.ExecContext(ctx, `
		UPDATE proxmox_nodes
		SET cpu_temp_source_host_id = $2,
		    fan_rpm_source_host_id = $2
		WHERE id = $1`, nodeID, hostID)
	return err
}

// ListProxmoxNodeCPUTempSourceCandidates returns hosts already linked to guests on this node.
func (db *DB) ListProxmoxNodeCPUTempSourceCandidates(ctx context.Context, connectionID, nodeName string) ([]models.Host, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT DISTINCT h.id, h.name, h.hostname, h.ip_address, h.os, h.agent_version,
		       '' AS api_key, h.tags::text, h.status, h.last_seen, h.created_at, h.updated_at
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN hosts h ON h.id = l.host_id
		WHERE g.connection_id = $1
		  AND g.node_name = $2
		  AND l.status = 'confirmed'
		ORDER BY h.name`, connectionID, nodeName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []models.Host{}
	for rows.Next() {
		var h models.Host
		var tagsJSON string
		if err := rows.Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.AgentVersion, &h.APIKey, &tagsJSON, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		h.Tags = parseTags(tagsJSON)
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetEffectiveHostCPUTemperature resolves CPU temperature for a host using Proxmox node mapping when relevant.
// Resolution order: mapped node source host temperature (fresh) -> fallbackLocal.
func (db *DB) GetEffectiveHostCPUTemperature(ctx context.Context, hostID string, fallbackLocal float64) (float64, bool) {
	var sourceHostID sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT n.cpu_temp_source_host_id
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN proxmox_nodes n ON n.connection_id = g.connection_id AND n.node_name = g.node_name
		WHERE l.host_id = $1
		  AND l.status = 'confirmed'
		  AND l.metrics_source IN ('auto', 'proxmox')
		LIMIT 1`, hostID).Scan(&sourceHostID)
	if err != nil && err != sql.ErrNoRows {
		if fallbackLocal > 0 {
			return fallbackLocal, true
		}
		return 0, false
	}

	if sourceHostID.Valid {
		var temp float64
		var ts time.Time
		err = db.conn.QueryRowContext(ctx, `
			SELECT cpu_temperature, timestamp
			FROM system_metrics
			WHERE host_id = $1
			ORDER BY timestamp DESC
			LIMIT 1`, sourceHostID.String).Scan(&temp, &ts)
		if err == nil && temp > 0 && time.Since(ts) <= 10*time.Minute {
			return temp, true
		}
	}

	if fallbackLocal > 0 {
		return fallbackLocal, true
	}
	return 0, false
}

// GetEffectiveHostFanRPM resolves fan RPM for a host using Proxmox node mapping when relevant.
// Resolution order: mapped node source host fan RPM (fresh) -> fallbackLocal.
func (db *DB) GetEffectiveHostFanRPM(ctx context.Context, hostID string, fallbackLocal float64) (float64, bool) {
	var sourceHostID sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT n.fan_rpm_source_host_id
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN proxmox_nodes n ON n.connection_id = g.connection_id AND n.node_name = g.node_name
		WHERE l.host_id = $1
		  AND l.status = 'confirmed'
		  AND l.metrics_source IN ('auto', 'proxmox')
		LIMIT 1`, hostID).Scan(&sourceHostID)
	if err != nil && err != sql.ErrNoRows {
		if fallbackLocal > 0 {
			return fallbackLocal, true
		}
		return 0, false
	}

	if sourceHostID.Valid {
		var rpm float64
		var ts time.Time
		err = db.conn.QueryRowContext(ctx, `
			SELECT COALESCE(fan_rpm, 0), timestamp
			FROM system_metrics
			WHERE host_id = $1
			ORDER BY timestamp DESC
			LIMIT 1`, sourceHostID.String).Scan(&rpm, &ts)
		if err == nil && rpm > 0 && time.Since(ts) <= 10*time.Minute {
			return rpm, true
		}
	}

	if fallbackLocal > 0 {
		return fallbackLocal, true
	}
	return 0, false
}

// GetProxmoxNodeCPUTemperatureHistory returns CPU temperature samples for the
// host mapped as the node's sensor source. Empty when no mapping exists.
func (db *DB) GetProxmoxNodeCPUTemperatureHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error) {
	var sourceHostID sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT n.cpu_temp_source_host_id
		FROM proxmox_nodes n
		WHERE n.id = $1`, nodeID).Scan(&sourceHostID)
	if err == sql.ErrNoRows || !sourceHostID.Valid || sourceHostID.String == "" {
		return []models.SystemMetrics{}, nil
	}
	if err != nil {
		return nil, err
	}

	return db.GetSystemCPUTemperatureHistoryByHost(ctx, sourceHostID.String, hours)
}

// GetProxmoxNodeFanRPMHistory returns fan RPM samples for the host mapped as
// the node's sensor source. Empty when no mapping exists.
func (db *DB) GetProxmoxNodeFanRPMHistory(ctx context.Context, nodeID string, hours int) ([]models.SystemMetrics, error) {
	var sourceHostID sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT n.fan_rpm_source_host_id
		FROM proxmox_nodes n
		WHERE n.id = $1`, nodeID).Scan(&sourceHostID)
	if err == sql.ErrNoRows || !sourceHostID.Valid || sourceHostID.String == "" {
		return []models.SystemMetrics{}, nil
	}
	if err != nil {
		return nil, err
	}

	return db.GetSystemFanRPMHistoryByHost(ctx, sourceHostID.String, hours)
}

// IsHostUsedAsProxmoxCPUTempSource returns true when the host is configured
// as CPU temperature source for at least one Proxmox node.
func (db *DB) IsHostUsedAsProxmoxCPUTempSource(ctx context.Context, hostID string) bool {
	var exists bool
	err := db.conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE cpu_temp_source_host_id = $1)`,
		hostID,
	).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

// IsHostUsedAsProxmoxFanRPMSource returns true when the host is configured
// as fan RPM source for at least one Proxmox node.
func (db *DB) IsHostUsedAsProxmoxFanRPMSource(ctx context.Context, hostID string) bool {
	var exists bool
	err := db.conn.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE fan_rpm_source_host_id = $1)`,
		hostID,
	).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}
