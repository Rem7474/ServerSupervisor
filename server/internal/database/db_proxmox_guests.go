package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// UpsertProxmoxGuest inserts or updates a VM/LXC record.
func (db *DB) UpsertProxmoxGuest(ctx context.Context, connectionID, nodeName, guestType string, vmid int, name, status string, cpuAlloc, cpuUsage float64, memAlloc, memUsage, diskAlloc, uptime int64, tags string) error {
	_, err := db.conn.ExecContext(ctx, `
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
func (db *DB) ListProxmoxGuestsByNode(ctx context.Context, connectionID, nodeName string) ([]models.ProxmoxGuest, error) {
	rows, err := db.conn.QueryContext(ctx, `
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
func (db *DB) ListProxmoxGuests(ctx context.Context, connectionID, guestType, status string) ([]models.ProxmoxGuest, error) {
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

	rows, err := db.conn.QueryContext(ctx, q, args...)
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
func (db *DB) DeleteStaleProxmoxGuests(ctx context.Context, connectionID string, cutoff time.Time) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM proxmox_guests WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

// DeleteStaleProxmoxNodes removes nodes not seen since the cutoff time.
func (db *DB) DeleteStaleProxmoxNodes(ctx context.Context, connectionID string, cutoff time.Time) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM proxmox_nodes WHERE connection_id=$1 AND last_seen_at < $2`,
		connectionID, cutoff)
	return err
}

// UpsertProxmoxStorage inserts or updates a storage record.
func (db *DB) UpsertProxmoxStorage(ctx context.Context, connectionID, nodeName, storageName, storageType string, total, used, avail int64, enabled, active, shared bool) error {
	_, err := db.conn.ExecContext(ctx, `
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
func (db *DB) ListProxmoxStoragesByNode(ctx context.Context, connectionID, nodeName string) ([]models.ProxmoxStorage, error) {
	rows, err := db.conn.QueryContext(ctx, `
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

// itoa converts an int to its decimal string representation. Used to build
// dynamic argument placeholders ($1, $2…) for the filter-driven list queries.
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
