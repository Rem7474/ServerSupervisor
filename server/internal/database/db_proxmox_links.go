package database

import (
	"database/sql"
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

// ─── Guest Links ──────────────────────────────────────────────────────────────

// UpsertProxmoxGuestLink creates or updates a link between a Proxmox guest and a host.
// If a link already exists for the given guest_id, it is updated only if the incoming
// status has higher priority (confirmed > suggested > ignored), or if the caller explicitly
// changes metrics_source.
func (db *DB) UpsertProxmoxGuestLink(guestID, hostID, status, metricsSource string) (*models.ProxmoxGuestLink, error) {
	if status == "" {
		status = "confirmed"
	}
	if metricsSource == "" {
		metricsSource = "auto"
	}

	var id string
	err := db.conn.QueryRow(`
		INSERT INTO proxmox_guest_links (guest_id, host_id, status, metrics_source)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (guest_id) DO UPDATE SET
		    host_id        = EXCLUDED.host_id,
		    status         = EXCLUDED.status,
		    metrics_source = EXCLUDED.metrics_source,
		    updated_at     = NOW()
		RETURNING id`,
		guestID, hostID, status, metricsSource,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	if status == "confirmed" {
		_ = db.setNodeCPUTempSourceIfUnset(guestID, hostID)
		_ = db.setNodeFanRPMSourceIfUnset(guestID, hostID)
	}
	return db.GetProxmoxGuestLink(id)
}

// AutoSuggestProxmoxLink tries to create a 'suggested' link for the given guest
// by matching the guest name against host hostnames and names.
// It does nothing if a link already exists for this guest.
func (db *DB) AutoSuggestProxmoxLink(guestID, guestName string) error {
	if guestName == "" {
		return nil
	}

	// Skip if a link already exists (in any status).
	var exists bool
	if err := db.conn.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM proxmox_guest_links WHERE guest_id = $1)`, guestID,
	).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Search for exactly one host that matches by hostname or name (case-insensitive).
	name := strings.ToLower(guestName)
	rows, err := db.conn.Query(`
		SELECT id FROM hosts
		WHERE LOWER(hostname) = $1 OR LOWER(name) = $1
		LIMIT 2`, name)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var hostID string
	count := 0
	for rows.Next() {
		count++
		if count == 1 {
			if err := rows.Scan(&hostID); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Only create a suggestion when there is exactly one unambiguous match.
	if count != 1 {
		return nil
	}

	_, err = db.conn.Exec(`
		INSERT INTO proxmox_guest_links (guest_id, host_id, status, metrics_source)
		VALUES ($1, $2, 'suggested', 'auto')
		ON CONFLICT (guest_id) DO NOTHING`,
		guestID, hostID,
	)
	return err
}

// GetProxmoxGuestLink returns a single link by its ID with joined display fields.
func (db *DB) GetProxmoxGuestLink(id string) (*models.ProxmoxGuestLink, error) {
	var l models.ProxmoxGuestLink
	err := db.conn.QueryRow(`
		SELECT l.id, l.guest_id, l.host_id, l.status, l.metrics_source, l.created_at, l.updated_at,
		       g.name, g.guest_type, g.node_name, g.vmid,
		       h.name, h.hostname,
		       g.cpu_usage, g.mem_alloc, g.mem_usage
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN hosts           h ON h.id = l.host_id
		WHERE l.id = $1`, id,
	).Scan(
		&l.ID, &l.GuestID, &l.HostID, &l.Status, &l.MetricsSource, &l.CreatedAt, &l.UpdatedAt,
		&l.GuestName, &l.GuestType, &l.NodeName, &l.VMID,
		&l.HostName, &l.HostHostname,
		&l.CPUUsage, &l.MemAlloc, &l.MemUsage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

// GetProxmoxGuestLinkByGuest returns the link for a specific Proxmox guest (if any).
func (db *DB) GetProxmoxGuestLinkByGuest(guestID string) (*models.ProxmoxGuestLink, error) {
	var l models.ProxmoxGuestLink
	err := db.conn.QueryRow(`
		SELECT l.id, l.guest_id, l.host_id, l.status, l.metrics_source, l.created_at, l.updated_at,
		       g.name, g.guest_type, g.node_name, g.vmid,
		       h.name, h.hostname,
		       g.cpu_usage, g.mem_alloc, g.mem_usage
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN hosts           h ON h.id = l.host_id
		WHERE l.guest_id = $1`, guestID,
	).Scan(
		&l.ID, &l.GuestID, &l.HostID, &l.Status, &l.MetricsSource, &l.CreatedAt, &l.UpdatedAt,
		&l.GuestName, &l.GuestType, &l.NodeName, &l.VMID,
		&l.HostName, &l.HostHostname,
		&l.CPUUsage, &l.MemAlloc, &l.MemUsage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

// GetProxmoxGuestLinkByHost returns the confirmed/suggested Proxmox link for a host (if any).
func (db *DB) GetProxmoxGuestLinkByHost(hostID string) (*models.ProxmoxGuestLink, error) {
	var l models.ProxmoxGuestLink
	err := db.conn.QueryRow(`
		SELECT l.id, l.guest_id, l.host_id, l.status, l.metrics_source, l.created_at, l.updated_at,
		       g.name, g.guest_type, g.node_name, g.vmid,
		       h.name, h.hostname,
		       g.cpu_usage, g.mem_alloc, g.mem_usage
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN hosts           h ON h.id = l.host_id
		WHERE l.host_id = $1 AND l.status != 'ignored'
		ORDER BY CASE l.status WHEN 'confirmed' THEN 0 ELSE 1 END
		LIMIT 1`, hostID,
	).Scan(
		&l.ID, &l.GuestID, &l.HostID, &l.Status, &l.MetricsSource, &l.CreatedAt, &l.UpdatedAt,
		&l.GuestName, &l.GuestType, &l.NodeName, &l.VMID,
		&l.HostName, &l.HostHostname,
		&l.CPUUsage, &l.MemAlloc, &l.MemUsage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &l, err
}

// ListProxmoxGuestLinks returns all links, optionally filtered by status.
func (db *DB) ListProxmoxGuestLinks(status string) ([]models.ProxmoxGuestLink, error) {
	q := `
		SELECT l.id, l.guest_id, l.host_id, l.status, l.metrics_source, l.created_at, l.updated_at,
		       g.name, g.guest_type, g.node_name, g.vmid,
		       h.name, h.hostname,
		       g.cpu_usage, g.mem_alloc, g.mem_usage
		FROM proxmox_guest_links l
		JOIN proxmox_guests g ON g.id = l.guest_id
		JOIN hosts           h ON h.id = l.host_id`
	args := []interface{}{}
	if status != "" {
		q += ` WHERE l.status = $1`
		args = append(args, status)
	}
	q += ` ORDER BY l.updated_at DESC`

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanGuestLinks(rows)
}

// UpdateProxmoxGuestLink updates status and/or metrics_source for a link.
func (db *DB) UpdateProxmoxGuestLink(id string, status, metricsSource *string) (*models.ProxmoxGuestLink, error) {
	_, err := db.conn.Exec(`
		UPDATE proxmox_guest_links SET
		    status         = COALESCE($2, status),
		    metrics_source = COALESCE($3, metrics_source),
		    updated_at     = NOW()
		WHERE id = $1`,
		id, status, metricsSource,
	)
	if err != nil {
		return nil, err
	}
	link, err := db.GetProxmoxGuestLink(id)
	if err != nil {
		return nil, err
	}
	if link != nil && link.Status == "confirmed" {
		_ = db.setNodeCPUTempSourceIfUnset(link.GuestID, link.HostID)
		_ = db.setNodeFanRPMSourceIfUnset(link.GuestID, link.HostID)
	}
	return link, nil
}

// DeleteProxmoxGuestLink removes a link by ID.
func (db *DB) DeleteProxmoxGuestLink(id string) error {
	_, err := db.conn.Exec(`DELETE FROM proxmox_guest_links WHERE id = $1`, id)
	return err
}

// GetProxmoxGuestIDByVMID returns the DB UUID for a guest identified by (connection_id, node_name, vmid).
func (db *DB) GetProxmoxGuestIDByVMID(connectionID, nodeName string, vmid int) (string, error) {
	var id string
	err := db.conn.QueryRow(`
		SELECT id FROM proxmox_guests WHERE connection_id=$1 AND node_name=$2 AND vmid=$3`,
		connectionID, nodeName, vmid,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// ListProxmoxLinkCandidates returns Proxmox guests that could be linked to the given host,
// ranked by name similarity. Used to populate the "link" dropdown from the host side.
func (db *DB) ListProxmoxLinkCandidates(hostID string) ([]models.ProxmoxGuest, error) {
	// Get the host hostname/name for matching
	var hostname, name string
	if err := db.conn.QueryRow(
		`SELECT LOWER(hostname), LOWER(name) FROM hosts WHERE id = $1`, hostID,
	).Scan(&hostname, &name); err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT g.id, g.connection_id, g.node_name, g.guest_type, g.vmid, g.name, g.status,
		       g.cpu_alloc, g.cpu_usage, g.mem_alloc, g.mem_usage, g.disk_alloc, g.tags, g.uptime, g.last_seen_at
		FROM proxmox_guests g
		LEFT JOIN proxmox_guest_links l ON l.guest_id = g.id
		WHERE l.id IS NULL
		   OR (l.host_id != $1 AND l.status = 'ignored')
		ORDER BY
		    CASE WHEN LOWER(g.name) = $2 OR LOWER(g.name) = $3 THEN 0 ELSE 1 END,
		    g.name
		LIMIT 50`,
		hostID, hostname, name,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanGuests(rows)
}

// IsProxmoxGuestDataFresh returns true when the confirmed 'auto' link for the given
// host has Proxmox guest data that was updated within 3× the connection's poll interval.
// Returns (false, nil) when no qualifying link exists (caller should treat Proxmox as absent).
func (db *DB) IsProxmoxGuestDataFresh(hostID string) (bool, error) {
	var fresh bool
	err := db.conn.QueryRow(`
		SELECT g.last_seen_at >= NOW() - (c.poll_interval_sec * 3 || ' seconds')::interval
		FROM proxmox_guest_links l
		JOIN proxmox_guests g      ON g.id = l.guest_id
		JOIN proxmox_connections c ON c.id = g.connection_id
		WHERE l.host_id = $1
		  AND l.status = 'confirmed'
		  AND l.metrics_source = 'auto'
		LIMIT 1
	`, hostID).Scan(&fresh)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return fresh, err
}

func scanGuestLinks(rows *sql.Rows) ([]models.ProxmoxGuestLink, error) {
	var links []models.ProxmoxGuestLink
	for rows.Next() {
		var l models.ProxmoxGuestLink
		if err := rows.Scan(
			&l.ID, &l.GuestID, &l.HostID, &l.Status, &l.MetricsSource, &l.CreatedAt, &l.UpdatedAt,
			&l.GuestName, &l.GuestType, &l.NodeName, &l.VMID,
			&l.HostName, &l.HostHostname,
			&l.CPUUsage, &l.MemAlloc, &l.MemUsage,
		); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	if links == nil {
		links = []models.ProxmoxGuestLink{}
	}
	return links, rows.Err()
}

func (db *DB) setNodeCPUTempSourceIfUnset(guestID, hostID string) error {
	_, err := db.conn.Exec(`
		UPDATE proxmox_nodes n
		SET cpu_temp_source_host_id = $2
		FROM proxmox_guests g
		WHERE g.id = $1
		  AND n.connection_id = g.connection_id
		  AND n.node_name = g.node_name
		  AND n.cpu_temp_source_host_id IS NULL`,
		guestID, hostID,
	)
	return err
}

func (db *DB) setNodeFanRPMSourceIfUnset(guestID, hostID string) error {
	_, err := db.conn.Exec(`
		UPDATE proxmox_nodes n
		SET fan_rpm_source_host_id = $2
		FROM proxmox_guests g
		WHERE g.id = $1
		  AND n.connection_id = g.connection_id
		  AND n.node_name = g.node_name
		  AND n.fan_rpm_source_host_id IS NULL`,
		guestID, hostID,
	)
	return err
}
