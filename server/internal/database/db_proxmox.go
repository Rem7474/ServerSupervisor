package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ─── Connections ──────────────────────────────────────────────────────────────

// CreateProxmoxConnection inserts a new connection and returns its ID.
func (db *DB) CreateProxmoxConnection(name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) (string, error) {
	if pollIntervalSec <= 0 {
		pollIntervalSec = 60
	}
	var id string
	err := db.conn.QueryRow(`
		INSERT INTO proxmox_connections (name, api_url, token_id, token_secret, insecure_skip_verify, enabled, poll_interval_sec)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		name, apiURL, tokenID, tokenSecret, insecureSkipVerify, enabled, pollIntervalSec,
	).Scan(&id)
	return id, err
}

// ListProxmoxConnections returns all connections without secrets.
// Node and guest counts are joined.
func (db *DB) ListProxmoxConnections() ([]models.ProxmoxConnection, error) {
	rows, err := db.conn.Query(`
		SELECT
			c.id, c.name, c.api_url, c.token_id,
			c.insecure_skip_verify, c.enabled, c.poll_interval_sec,
			c.last_error, c.last_error_at, c.last_success_at,
			c.created_at, c.updated_at,
			COUNT(DISTINCT n.id) AS node_count,
			COUNT(DISTINCT g.id) AS guest_count
		FROM proxmox_connections c
		LEFT JOIN proxmox_nodes   n ON n.connection_id = c.id
		LEFT JOIN proxmox_guests  g ON g.connection_id = c.id
		GROUP BY c.id
		ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var conns []models.ProxmoxConnection
	for rows.Next() {
		var c models.ProxmoxConnection
		var lastErrAt, lastSuccAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.TokenID,
			&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
			&c.LastError, &lastErrAt, &lastSuccAt,
			&c.CreatedAt, &c.UpdatedAt,
			&c.NodeCount, &c.GuestCount,
		); err != nil {
			return nil, err
		}
		if lastErrAt.Valid {
			t := lastErrAt.Time
			c.LastErrorAt = &t
		}
		if lastSuccAt.Valid {
			t := lastSuccAt.Time
			c.LastSuccessAt = &t
		}
		conns = append(conns, c)
	}
	if conns == nil {
		conns = []models.ProxmoxConnection{}
	}
	return conns, rows.Err()
}

// GetProxmoxConnectionByID returns a connection without secret.
func (db *DB) GetProxmoxConnectionByID(id string) (*models.ProxmoxConnection, error) {
	var c models.ProxmoxConnection
	var lastErrAt, lastSuccAt sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, name, api_url, token_id, insecure_skip_verify, enabled, poll_interval_sec,
		       last_error, last_error_at, last_success_at, created_at, updated_at
		FROM proxmox_connections WHERE id = $1`, id).Scan(
		&c.ID, &c.Name, &c.APIURL, &c.TokenID,
		&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
		&c.LastError, &lastErrAt, &lastSuccAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastErrAt.Valid {
		t := lastErrAt.Time
		c.LastErrorAt = &t
	}
	if lastSuccAt.Valid {
		t := lastSuccAt.Time
		c.LastSuccessAt = &t
	}
	return &c, nil
}

// GetEnabledProxmoxConnections returns enabled connections WITH their secrets (for the poller).
type ProxmoxConnectionFull struct {
	models.ProxmoxConnection
	TokenSecret string
}

func (db *DB) GetEnabledProxmoxConnections() ([]ProxmoxConnectionFull, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, api_url, token_id, token_secret,
		       insecure_skip_verify, enabled, poll_interval_sec,
		       last_error, last_error_at, last_success_at, created_at, updated_at
		FROM proxmox_connections WHERE enabled = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var conns []ProxmoxConnectionFull
	for rows.Next() {
		var c ProxmoxConnectionFull
		var lastErrAt, lastSuccAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.TokenID, &c.TokenSecret,
			&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
			&c.LastError, &lastErrAt, &lastSuccAt,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastErrAt.Valid {
			t := lastErrAt.Time
			c.LastErrorAt = &t
		}
		if lastSuccAt.Valid {
			t := lastSuccAt.Time
			c.LastSuccessAt = &t
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

// UpdateProxmoxConnection updates mutable fields.
// If tokenSecret is empty the existing secret is preserved.
func (db *DB) UpdateProxmoxConnection(id, name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) error {
	if pollIntervalSec <= 0 {
		pollIntervalSec = 60
	}
	if tokenSecret == "" {
		_, err := db.conn.Exec(`
			UPDATE proxmox_connections
			SET name=$2, api_url=$3, token_id=$4,
			    insecure_skip_verify=$5, enabled=$6, poll_interval_sec=$7, updated_at=NOW()
			WHERE id=$1`,
			id, name, apiURL, tokenID, insecureSkipVerify, enabled, pollIntervalSec)
		return err
	}
	_, err := db.conn.Exec(`
		UPDATE proxmox_connections
		SET name=$2, api_url=$3, token_id=$4, token_secret=$5,
		    insecure_skip_verify=$6, enabled=$7, poll_interval_sec=$8, updated_at=NOW()
		WHERE id=$1`,
		id, name, apiURL, tokenID, tokenSecret, insecureSkipVerify, enabled, pollIntervalSec)
	return err
}

// DeleteProxmoxConnection removes a connection (cascade deletes nodes/guests/storages).
func (db *DB) DeleteProxmoxConnection(id string) error {
	_, err := db.conn.Exec(`DELETE FROM proxmox_connections WHERE id = $1`, id)
	return err
}

// UpdateProxmoxConnectionSuccess records a successful poll.
func (db *DB) UpdateProxmoxConnectionSuccess(id string) error {
	_, err := db.conn.Exec(`
		UPDATE proxmox_connections SET last_error='', last_error_at=NULL, last_success_at=NOW(), updated_at=NOW()
		WHERE id=$1`, id)
	return err
}

// UpdateProxmoxConnectionError records a poll error.
func (db *DB) UpdateProxmoxConnectionError(id, errMsg string) error {
	_, err := db.conn.Exec(`
		UPDATE proxmox_connections SET last_error=$2, last_error_at=NOW(), updated_at=NOW()
		WHERE id=$1`, id, errMsg)
	return err
}

// GetProxmoxTokenSecret returns only the token secret for a connection.
func (db *DB) GetProxmoxTokenSecret(id string) (string, error) {
	var secret string
	err := db.conn.QueryRow(`SELECT token_secret FROM proxmox_connections WHERE id=$1`, id).Scan(&secret)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return secret, err
}

// ─── Nodes ────────────────────────────────────────────────────────────────────

// UpsertProxmoxNode inserts or updates a node record.
func (db *DB) UpsertProxmoxNode(connectionID, nodeName, status string, cpuCount int, cpuUsage float64, memTotal, memUsed, uptime int64, pveVersion, clusterName, ipAddress string) error {
	_, err := db.conn.Exec(`
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
	n.cpu_temp_source_host_id,
	MAX(COALESCE(src.hostname, src.name, '')) AS cpu_temp_source_host_name,
	n.cpu_count, n.cpu_usage, n.mem_total, n.mem_used,
	n.uptime, n.pve_version, n.cluster_name, n.ip_address, n.last_seen_at,
	n.pending_updates, n.security_updates, n.last_update_check_at,
	COUNT(CASE WHEN g.guest_type='vm'  THEN 1 END) AS vm_count,
	COUNT(CASE WHEN g.guest_type='lxc' THEN 1 END) AS lxc_count`

// ListProxmoxNodes returns all nodes with VM/LXC counts.
func (db *DB) ListProxmoxNodes() ([]models.ProxmoxNode, error) {
	rows, err := db.conn.Query(`
		SELECT ` + nodeSelectCols + `
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
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
func (db *DB) ListProxmoxNodesByConnection(connectionID string) ([]models.ProxmoxNode, error) {
	rows, err := db.conn.Query(`
		SELECT `+nodeSelectCols+`
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
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
func (db *DB) GetProxmoxNode(id string) (*models.ProxmoxNode, error) {
	var n models.ProxmoxNode
	var lastUpdateCheckAt sql.NullTime
	err := db.conn.QueryRow(`
		SELECT `+nodeSelectCols+`
		FROM proxmox_nodes n
		LEFT JOIN hosts src         ON src.id = n.cpu_temp_source_host_id
		LEFT JOIN proxmox_guests g ON g.connection_id=n.connection_id AND g.node_name=n.node_name
		WHERE n.id=$1
		GROUP BY n.id`, id).Scan(
		&n.ID, &n.ConnectionID, &n.NodeName, &n.Status,
		&n.CPUTempSourceHostID, &n.CPUTempSourceHostName,
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
	guests, err := db.ListProxmoxGuestsByNode(n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Guests = guests

	// Load storages
	storages, err := db.ListProxmoxStoragesByNode(n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Storages = storages

	// Load physical disks
	disks, err := db.ListProxmoxDisksByNode(n.ConnectionID, n.NodeName)
	if err != nil {
		return nil, err
	}
	n.Disks = disks

	// Load 50 most recent tasks
	tasks, err := db.ListProxmoxTasksByNode(n.ConnectionID, n.NodeName, 50)
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

// SetProxmoxNodeCPUTempSource maps a Proxmox node to a host used as CPU temperature source.
// Pass an empty hostID to clear the mapping.
func (db *DB) SetProxmoxNodeCPUTempSource(nodeID, hostID string) error {
	if hostID == "" {
		_, err := db.conn.Exec(`UPDATE proxmox_nodes SET cpu_temp_source_host_id = NULL WHERE id = $1`, nodeID)
		return err
	}
	_, err := db.conn.Exec(`UPDATE proxmox_nodes SET cpu_temp_source_host_id = $2 WHERE id = $1`, nodeID, hostID)
	return err
}

// ListProxmoxNodeCPUTempSourceCandidates returns hosts already linked to guests on this node.
func (db *DB) ListProxmoxNodeCPUTempSourceCandidates(connectionID, nodeName string) ([]models.Host, error) {
	rows, err := db.conn.Query(`
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
func (db *DB) GetEffectiveHostCPUTemperature(hostID string, fallbackLocal float64) (float64, bool) {
	var sourceHostID sql.NullString
	err := db.conn.QueryRow(`
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
		err = db.conn.QueryRow(`
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

// IsHostUsedAsProxmoxCPUTempSource returns true when the host is configured
// as CPU temperature source for at least one Proxmox node.
func (db *DB) IsHostUsedAsProxmoxCPUTempSource(hostID string) bool {
	var exists bool
	err := db.conn.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE cpu_temp_source_host_id = $1)`,
		hostID,
	).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

// ─── Guests ───────────────────────────────────────────────────────────────────

// UpsertProxmoxGuest inserts or updates a VM/LXC record.
func (db *DB) UpsertProxmoxGuest(connectionID, nodeName, guestType string, vmid int, name, status string, cpuAlloc, cpuUsage float64, memAlloc, memUsage, diskAlloc, uptime int64, tags string) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_guests
		    (connection_id, node_name, guest_type, vmid, name, status,
		     cpu_alloc, cpu_usage, mem_alloc, mem_usage, disk_alloc, tags, uptime, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (connection_id, node_name, vmid) DO UPDATE SET
		    guest_type  = EXCLUDED.guest_type,
		    name        = EXCLUDED.name,
		    status      = EXCLUDED.status,
		    cpu_alloc   = EXCLUDED.cpu_alloc,
		    cpu_usage   = EXCLUDED.cpu_usage,
		    mem_alloc   = EXCLUDED.mem_alloc,
		    mem_usage   = EXCLUDED.mem_usage,
		    disk_alloc  = EXCLUDED.disk_alloc,
		    tags        = EXCLUDED.tags,
		    uptime      = EXCLUDED.uptime,
		    last_seen_at = NOW()`,
		connectionID, nodeName, guestType, vmid, name, status,
		cpuAlloc, cpuUsage, memAlloc, memUsage, diskAlloc, tags, uptime,
	)
	return err
}

// ListProxmoxGuestsByNode returns all guests for a given node (within a connection).
func (db *DB) ListProxmoxGuestsByNode(connectionID, nodeName string) ([]models.ProxmoxGuest, error) {
	rows, err := db.conn.Query(`
		SELECT id, connection_id, node_name, guest_type, vmid, name, status,
		       cpu_alloc, cpu_usage, mem_alloc, mem_usage, disk_alloc, tags, uptime, last_seen_at
		FROM proxmox_guests
		WHERE connection_id=$1 AND node_name=$2
		ORDER BY guest_type, vmid`, connectionID, nodeName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanGuests(rows)
}

// ListProxmoxGuests returns all guests with optional filters.
// Pass empty strings to skip filters.
func (db *DB) ListProxmoxGuests(connectionID, guestType, status string) ([]models.ProxmoxGuest, error) {
	q := `SELECT id, connection_id, node_name, guest_type, vmid, name, status,
		         cpu_alloc, cpu_usage, mem_alloc, mem_usage, disk_alloc, tags, uptime, last_seen_at
		  FROM proxmox_guests WHERE 1=1`
	args := []interface{}{}
	n := 1
	if connectionID != "" {
		q += ` AND connection_id=$` + itoa(n)
		args = append(args, connectionID)
		n++
	}
	if guestType != "" {
		q += ` AND guest_type=$` + itoa(n)
		args = append(args, guestType)
		n++
	}
	if status != "" {
		q += ` AND status=$` + itoa(n)
		args = append(args, status)
	}
	q += ` ORDER BY guest_type, node_name, vmid`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanGuests(rows)
}

func scanGuests(rows *sql.Rows) ([]models.ProxmoxGuest, error) {
	var guests []models.ProxmoxGuest
	for rows.Next() {
		var g models.ProxmoxGuest
		if err := rows.Scan(
			&g.ID, &g.ConnectionID, &g.NodeName, &g.GuestType, &g.VMID, &g.Name, &g.Status,
			&g.CPUAlloc, &g.CPUUsage, &g.MemAlloc, &g.MemUsage, &g.DiskAlloc, &g.Tags, &g.Uptime,
			&g.LastSeenAt,
		); err != nil {
			return nil, err
		}
		guests = append(guests, g)
	}
	if guests == nil {
		guests = []models.ProxmoxGuest{}
	}
	return guests, rows.Err()
}

// DeleteStaleProxmoxGuests removes guests not seen since the cutoff time for a connection.
func (db *DB) DeleteStaleProxmoxGuests(connectionID string, cutoff time.Time) error {
	_, err := db.conn.Exec(`DELETE FROM proxmox_guests WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

// DeleteStaleProxmoxNodes removes nodes not seen since the cutoff time.
func (db *DB) DeleteStaleProxmoxNodes(connectionID string, cutoff time.Time) error {
	_, err := db.conn.Exec(`DELETE FROM proxmox_nodes WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

// ─── Storages ─────────────────────────────────────────────────────────────────

// UpsertProxmoxStorage inserts or updates a storage record.
func (db *DB) UpsertProxmoxStorage(connectionID, nodeName, storageName, storageType string, total, used, avail int64, enabled, active, shared bool) error {
	_, err := db.conn.Exec(`
		INSERT INTO proxmox_storages
		    (connection_id, node_name, storage_name, storage_type, total, used, avail, enabled, active, shared, last_seen_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())
		ON CONFLICT (connection_id, node_name, storage_name) DO UPDATE SET
		    storage_type = EXCLUDED.storage_type,
		    total        = EXCLUDED.total,
		    used         = EXCLUDED.used,
		    avail        = EXCLUDED.avail,
		    enabled      = EXCLUDED.enabled,
		    active       = EXCLUDED.active,
		    shared       = EXCLUDED.shared,
		    last_seen_at = NOW()`,
		connectionID, nodeName, storageName, storageType, total, used, avail, enabled, active, shared,
	)
	return err
}

// ListProxmoxStoragesByNode returns storages for a given node.
func (db *DB) ListProxmoxStoragesByNode(connectionID, nodeName string) ([]models.ProxmoxStorage, error) {
	rows, err := db.conn.Query(`
		SELECT id, connection_id, node_name, storage_name, storage_type,
		       total, used, avail, enabled, active, shared, last_seen_at
		FROM proxmox_storages
		WHERE connection_id=$1 AND node_name=$2
		ORDER BY storage_name`, connectionID, nodeName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanStorages(rows)
}

// GetProxmoxSummary returns aggregate stats and health signals across all connections.
func (db *DB) GetProxmoxSummary() (models.ProxmoxSummary, error) {
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
		if err := db.conn.QueryRow(q.q).Scan(q.dest); err != nil {
			return s, err
		}
	}

	err := db.conn.QueryRow(`SELECT COALESCE(SUM(total),0), COALESCE(SUM(used),0) FROM proxmox_storages`).
		Scan(&s.StorageTotal, &s.StorageUsed)
	return s, err
}

// GetMaxProxmoxStorageUsagePercent returns the max used/total ratio (0-100) across all active Proxmox storages.
// Returns 0 if no storage data is available.
func (db *DB) GetMaxProxmoxStorageUsagePercent() float64 {
	var pct float64
	_ = db.conn.QueryRow(`
		SELECT COALESCE(MAX(used::float / NULLIF(total,0) * 100), 0)
		FROM proxmox_storages
		WHERE total > 0 AND enabled = TRUE AND active = TRUE
	`).Scan(&pct)
	return pct
}

func scanStorages(rows *sql.Rows) ([]models.ProxmoxStorage, error) {
	var storages []models.ProxmoxStorage
	for rows.Next() {
		var s models.ProxmoxStorage
		if err := rows.Scan(
			&s.ID, &s.ConnectionID, &s.NodeName, &s.StorageName, &s.StorageType,
			&s.Total, &s.Used, &s.Avail, &s.Enabled, &s.Active, &s.Shared, &s.LastSeenAt,
		); err != nil {
			return nil, err
		}
		storages = append(storages, s)
	}
	if storages == nil {
		storages = []models.ProxmoxStorage{}
	}
	return storages, rows.Err()
}

// itoa converts an int to its decimal string representation.
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
