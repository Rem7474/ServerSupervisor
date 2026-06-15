package database

import (
	"context"

	"github.com/serversupervisor/server/internal/models"
)

// scopeOptions runs a `SELECT id, label` query and collects the rows.
func (db *DB) scopeOptions(ctx context.Context, query string, args ...interface{}) ([]models.AlertScopeOption, error) {
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.AlertScopeOption{}
	for rows.Next() {
		var o models.AlertScopeOption
		if err := rows.Scan(&o.ID, &o.Label); err != nil {
			continue
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ===== Proxmox scope option lists (for alert-rule scope selectors) =====

func (db *DB) ListAlertProxmoxConnections(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `SELECT id, name FROM proxmox_connections ORDER BY name`)
}

func (db *DB) ListAlertProxmoxNodes(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `
		SELECT n.id, COALESCE(c.name,'?') || ' / ' || n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		ORDER BY c.name, n.node_name`)
}

func (db *DB) ListAlertProxmoxStorages(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `
		SELECT s.id, COALESCE(c.name,'?') || ' / ' || s.node_name || ' / ' || s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		ORDER BY c.name, s.node_name, s.storage_name`)
}

func (db *DB) ListAlertProxmoxGuests(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `
		SELECT g.id,
		       COALESCE(c.name,'?') || ' / ' || g.node_name || ' / ' || COALESCE(NULLIF(g.name,''), '(sans nom)') || ' (' || UPPER(g.guest_type) || ':' || g.vmid || ')'
		FROM proxmox_guests g
		LEFT JOIN proxmox_connections c ON c.id = g.connection_id
		ORDER BY c.name, g.node_name, g.guest_type, g.vmid`)
}

func (db *DB) ListAlertProxmoxDisks(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `
		SELECT d.id,
		       COALESCE(c.name,'?') || ' / ' || d.node_name || ' / ' ||
		       CASE
		         WHEN COALESCE(NULLIF(d.model,''),'') <> '' THEN d.model || ' (' || d.dev_path || ')'
		         ELSE d.dev_path
		       END
		FROM proxmox_disks d
		LEFT JOIN proxmox_connections c ON c.id = d.connection_id
		ORDER BY c.name, d.node_name, d.dev_path`)
}

// ListAlertDockerScopeHosts returns the hosts that currently have Docker
// containers (id + hostname), for the Docker alert scope selector.
func (db *DB) ListAlertDockerScopeHosts(ctx context.Context) ([]models.AlertScopeOption, error) {
	return db.scopeOptions(ctx, `
		SELECT DISTINCT dc.host_id, h.hostname
		FROM docker_containers dc
		JOIN hosts h ON h.id = dc.host_id
		ORDER BY h.hostname`)
}

// ===== Proxmox scope test-target label parts (for the rule test preview) =====

func (db *DB) ProxmoxConnectionName(ctx context.Context, id string) (string, error) {
	var name string
	err := db.conn.QueryRowContext(ctx, `SELECT name FROM proxmox_connections WHERE id = $1`, id).Scan(&name)
	return name, err
}

func (db *DB) ProxmoxNodeLabelParts(ctx context.Context, id string) (connName, nodeName string, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		WHERE n.id = $1`, id).Scan(&connName, &nodeName)
	return connName, nodeName, err
}

func (db *DB) ProxmoxStorageLabelParts(ctx context.Context, id string) (connName, nodeName, storageName string, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), s.node_name, s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		WHERE s.id = $1`, id).Scan(&connName, &nodeName, &storageName)
	return connName, nodeName, storageName, err
}

func (db *DB) ProxmoxGuestLabelParts(ctx context.Context, id string) (connName, nodeName, guestName, guestType string, vmid int, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), g.node_name, g.name, g.guest_type, g.vmid
		FROM proxmox_guests g
		LEFT JOIN proxmox_connections c ON c.id = g.connection_id
		WHERE g.id = $1`, id).Scan(&connName, &nodeName, &guestName, &guestType, &vmid)
	return connName, nodeName, guestName, guestType, vmid, err
}

func (db *DB) ProxmoxDiskLabelParts(ctx context.Context, id string) (connName, nodeName, devPath, model string, err error) {
	err = db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), d.node_name, d.dev_path, d.model
		FROM proxmox_disks d
		LEFT JOIN proxmox_connections c ON c.id = d.connection_id
		WHERE d.id = $1`, id).Scan(&connName, &nodeName, &devPath, &model)
	return connName, nodeName, devPath, model, err
}
