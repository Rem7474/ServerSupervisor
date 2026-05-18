// Package database — unified notification feed (alert incidents + release
// tracker executions) plus the proxmox-label resolvers used to enrich each
// entry with a human-readable source ("Noeud Proxmox sur conn / node-01").
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

// GetRecentNotifications returns the latest alert incidents with enriched metadata
// for WebSocket browser notification delivery.
func (db *DB) GetRecentNotifications(ctx context.Context, limit int) ([]models.NotificationItem, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT * FROM (
			SELECT
				'alert:' || ai.id::text AS id,
				'alert_incident'::text AS type,
				ai.rule_id,
				ai.host_id,
				COALESCE(h.name, ai.host_id) AS host_name,
				COALESCE(ar.name,
					ar.metric || ' ' || ar.operator || ' ' || CAST(COALESCE(ar.threshold_crit, ar.threshold_warn, 0) AS TEXT),
					'Règle supprimée') AS rule_name,
				COALESCE(ar.metric, '') AS metric,
				COALESCE(ai.severity, '') AS severity,
				''::text AS status,
				''::text AS tracker_id,
				''::text AS tracker_type,
				''::text AS release_url,
				''::text AS release_name,
				''::text AS version,
				ai.value,
				ai.triggered_at,
				ai.resolved_at,
				COALESCE(ar.actions->'channels' @> '["browser"]'::jsonb, FALSE) AS browser_notify
			FROM alert_incidents ai
			LEFT JOIN alert_rules ar ON ai.rule_id = ar.id
			LEFT JOIN hosts h ON ai.host_id = h.id

			UNION ALL

			SELECT
				'tracker:' || rte.id::text AS id,
				CASE
					WHEN rte.status IN ('pending', 'running') THEN 'release_tracker_detected'
					ELSE 'release_tracker_execution'
				END AS type,
				NULL::bigint AS rule_id,
				COALESCE(rt.host_id, '') AS host_id,
				COALESCE(h.name, rt.host_id, 'Source inconnue') AS host_name,
				COALESCE(rt.name, 'Release tracker') AS rule_name,
				'release_tracker'::text AS metric,
				''::text AS severity,
				COALESCE(rte.status, '') AS status,
				COALESCE(rt.id::text, '') AS tracker_id,
				COALESCE(rt.tracker_type, '') AS tracker_type,
				COALESCE(rte.release_url, '') AS release_url,
				COALESCE(rte.release_name, '') AS release_name,
				COALESCE(NULLIF(rte.tag_name, ''), '') AS version,
				0::double precision AS value,
				rte.triggered_at,
				rte.completed_at AS resolved_at,
				TRUE AS browser_notify
			FROM release_tracker_executions rte
			JOIN release_trackers rt ON rte.tracker_id = rt.id
			LEFT JOIN hosts h ON rt.host_id = h.id
			WHERE rt.notify_channels @> ARRAY['browser']::text[]
		) AS unified
		ORDER BY unified.triggered_at DESC
		LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []models.NotificationItem
	for rows.Next() {
		var item models.NotificationItem
		var ruleID sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.Type, &ruleID, &item.HostID,
			&item.HostName, &item.RuleName, &item.Metric, &item.Severity,
			&item.Status, &item.TrackerID, &item.TrackerType,
			&item.ReleaseURL, &item.ReleaseName, &item.Version,
			&item.Value, &item.TriggeredAt, &item.ResolvedAt,
			&item.BrowserNotify,
		); err != nil {
			continue
		}
		if ruleID.Valid {
			item.RuleID = &ruleID.Int64
		}
		db.enrichNotificationSource(ctx, &item)
		items = append(items, item)
	}
	return items, nil
}

// enrichNotificationSource fills SourceType/SourceLabel/HostName so a
// notification rendered in the UI shows the real Proxmox scope (node, guest,
// storage, disk, connection, global) rather than the opaque "proxmox:..." key.
func (db *DB) enrichNotificationSource(ctx context.Context, item *models.NotificationItem) {
	if item == nil {
		return
	}
	if item.Type == "" {
		item.Type = "alert_incident"
	}
	if strings.HasPrefix(item.Type, "release_tracker") {
		item.SourceType = "release_tracker"
		if item.TrackerType == "docker" {
			item.SourceLabel = "Docker tracker"
		} else {
			item.SourceLabel = "Git tracker"
		}
		return
	}
	item.SourceType = "agent"
	item.SourceLabel = item.HostName

	if !strings.HasPrefix(item.HostID, "proxmox:") {
		return
	}

	item.SourceType = "proxmox"
	parts := strings.SplitN(item.HostID, ":", 3)
	if len(parts) < 2 {
		item.SourceLabel = "Proxmox"
		return
	}

	scope := parts[1]
	rawID := ""
	if len(parts) == 3 {
		rawID = parts[2]
	}

	switch scope {
	case "node":
		name, ctx := db.resolveProxmoxNodeInfo(ctx, rawID)
		item.HostName = name
		item.SourceLabel = ctx
		return
	case "guest":
		name, ctx := db.resolveProxmoxGuestInfo(ctx, rawID)
		item.HostName = name
		item.SourceLabel = ctx
		return
	case "storage":
		name, ctx := db.resolveProxmoxStorageInfo(ctx, rawID)
		item.HostName = name
		item.SourceLabel = ctx
		return
	case "disk":
		name, ctx := db.resolveProxmoxDiskInfo(ctx, rawID)
		item.HostName = name
		item.SourceLabel = ctx
		return
	case "connection":
		label := db.resolveProxmoxConnectionLabel(ctx, rawID)
		item.HostName = label
		item.SourceLabel = "Proxmox"
		return
	case "global":
		if label := db.resolveProxmoxGlobalLikelySource(ctx, item.Metric); label != "" {
			item.HostName = label
			item.SourceLabel = label + " (source actuelle)"
		}
		return
	}

	item.HostName = "Proxmox"
	item.SourceLabel = "Proxmox"
}

func (db *DB) resolveProxmoxConnectionLabel(ctx context.Context, connectionID string) string {
	if connectionID == "" {
		return "Proxmox cluster"
	}
	var name string
	if err := db.conn.QueryRowContext(ctx, `SELECT name FROM proxmox_connections WHERE id = $1`, connectionID).Scan(&name); err != nil {
		return "Connexion " + connectionID
	}
	return "Connexion " + name
}

func (db *DB) resolveProxmoxNodeLabel(ctx context.Context, nodeID string) string {
	if nodeID == "" {
		return "Noeud Proxmox"
	}
	var connName, nodeName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		WHERE n.id = $1`, nodeID).Scan(&connName, &nodeName)
	if err != nil {
		return "Noeud " + nodeID
	}
	if strings.TrimSpace(connName) != "" {
		return "Noeud " + connName + " / " + nodeName
	}
	return "Noeud " + nodeName
}

func (db *DB) resolveProxmoxGuestLabel(ctx context.Context, guestID string) string {
	if guestID == "" {
		return "VM/LXC Proxmox"
	}
	var connName, nodeName, guestName, guestType string
	var vmid int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), g.node_name, COALESCE(NULLIF(g.name,''), '(sans nom)'), g.guest_type, g.vmid
		FROM proxmox_guests g
		LEFT JOIN proxmox_connections c ON c.id = g.connection_id
		WHERE g.id = $1`, guestID).Scan(&connName, &nodeName, &guestName, &guestType, &vmid)
	if err != nil {
		return "VM/LXC " + guestID
	}
	base := fmt.Sprintf("VM/LXC %s (%s:%d)", guestName, strings.ToUpper(guestType), vmid)
	if strings.TrimSpace(connName) != "" {
		return base + " sur " + connName + " / " + nodeName
	}
	return base + " sur " + nodeName
}

func (db *DB) resolveProxmoxStorageLabel(ctx context.Context, storageID string) string {
	if storageID == "" {
		return "Stockage Proxmox"
	}
	var connName, nodeName, storageName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), s.node_name, s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		WHERE s.id = $1`, storageID).Scan(&connName, &nodeName, &storageName)
	if err != nil {
		return "Stockage " + storageID
	}
	if strings.TrimSpace(connName) != "" {
		return "Stockage " + connName + " / " + nodeName + " / " + storageName
	}
	return "Stockage " + nodeName + " / " + storageName
}

func (db *DB) resolveProxmoxDiskLabel(ctx context.Context, diskID string) string {
	if diskID == "" {
		return "Disque Proxmox"
	}
	var connName, nodeName, devPath, model string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), d.node_name, d.dev_path, COALESCE(d.model, '')
		FROM proxmox_disks d
		LEFT JOIN proxmox_connections c ON c.id = d.connection_id
		WHERE d.id = $1`, diskID).Scan(&connName, &nodeName, &devPath, &model)
	if err != nil {
		return "Disque " + diskID
	}
	detail := devPath
	if strings.TrimSpace(model) != "" {
		detail = model + " (" + devPath + ")"
	}
	if strings.TrimSpace(connName) != "" {
		return "Disque " + connName + " / " + nodeName + " / " + detail
	}
	return "Disque " + nodeName + " / " + detail
}

func (db *DB) resolveProxmoxNodeInfo(ctx context.Context, nodeID string) (name, context string) {
	if nodeID == "" {
		return "Noeud Proxmox", "Proxmox"
	}
	var connName, nodeName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		WHERE n.id = $1`, nodeID).Scan(&connName, &nodeName)
	if err != nil {
		return "Noeud " + nodeID, "Proxmox"
	}
	name = nodeName
	if strings.TrimSpace(connName) != "" {
		context = "Noeud Proxmox sur " + connName
	} else {
		context = "Noeud Proxmox"
	}
	return
}

func (db *DB) resolveProxmoxGuestInfo(ctx context.Context, guestID string) (name, context string) {
	if guestID == "" {
		return "VM/LXC Proxmox", "Proxmox"
	}
	var connName, nodeName, guestName, guestType string
	var vmid int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), g.node_name, COALESCE(NULLIF(g.name, ''), '(sans nom)'), g.guest_type, g.vmid
		FROM proxmox_guests g
		LEFT JOIN proxmox_connections c ON c.id = g.connection_id
		WHERE g.id = $1`, guestID).Scan(&connName, &nodeName, &guestName, &guestType, &vmid)
	if err != nil {
		return "VM/LXC " + guestID, "Proxmox"
	}
	name = fmt.Sprintf("%s (%s:%d)", guestName, strings.ToUpper(guestType), vmid)
	if strings.TrimSpace(connName) != "" {
		context = "Proxmox VM/LXC sur " + connName + " / " + nodeName
	} else {
		context = "Proxmox VM/LXC sur " + nodeName
	}
	return
}

func (db *DB) resolveProxmoxStorageInfo(ctx context.Context, storageID string) (name, context string) {
	if storageID == "" {
		return "Stockage Proxmox", "Proxmox"
	}
	var connName, nodeName, storageName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), s.node_name, s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		WHERE s.id = $1`, storageID).Scan(&connName, &nodeName, &storageName)
	if err != nil {
		return "Stockage " + storageID, "Proxmox"
	}
	name = storageName
	if strings.TrimSpace(connName) != "" {
		context = "Stockage Proxmox sur " + connName + " / " + nodeName
	} else {
		context = "Stockage Proxmox sur " + nodeName
	}
	return
}

func (db *DB) resolveProxmoxDiskInfo(ctx context.Context, diskID string) (name, context string) {
	if diskID == "" {
		return "Disque Proxmox", "Proxmox"
	}
	var connName, nodeName, devPath, model string
	err := db.conn.QueryRowContext(ctx, `
		SELECT COALESCE(c.name, ''), d.node_name, d.dev_path, COALESCE(d.model, '')
		FROM proxmox_disks d
		LEFT JOIN proxmox_connections c ON c.id = d.connection_id
		WHERE d.id = $1`, diskID).Scan(&connName, &nodeName, &devPath, &model)
	if err != nil {
		return "Disque " + diskID, "Proxmox"
	}
	if strings.TrimSpace(model) != "" {
		name = model + " (" + devPath + ")"
	} else {
		name = devPath
	}
	if strings.TrimSpace(connName) != "" {
		context = "Disque Proxmox sur " + connName + " / " + nodeName
	} else {
		context = "Disque Proxmox sur " + nodeName
	}
	return
}

// resolveProxmoxGlobalLikelySource picks the most representative Proxmox
// object for a "global" alert scope, so the user sees e.g. "the node that
// triggered the cluster-wide CPU alarm" instead of just "Cluster Proxmox".
func (db *DB) resolveProxmoxGlobalLikelySource(ctx context.Context, metric string) string {
	switch metric {
	case "proxmox_node_cpu_percent", "proxmox_node_memory_percent", "proxmox_node_cpu_temperature", "proxmox_node_fan_rpm", "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_auth_failures_recent":
		var nodeID string
		if err := db.conn.QueryRowContext(ctx, `
			SELECT n.id
			FROM proxmox_nodes n
			LEFT JOIN LATERAL (
				SELECT COUNT(*) AS failed_tasks
				FROM proxmox_tasks t
				WHERE t.connection_id = n.connection_id
				  AND t.node_name = n.node_name
				  AND t.status='stopped'
				  AND t.exit_status != ''
				  AND t.exit_status != 'OK'
				  AND t.start_time >= NOW() - INTERVAL '24 hours'
			) ft ON TRUE
			ORDER BY CASE
				WHEN $1 = 'proxmox_node_memory_percent' THEN CASE WHEN n.mem_total > 0 THEN n.mem_used::float / n.mem_total ELSE 0 END
				WHEN $1 = 'proxmox_node_pending_updates' THEN n.pending_updates::float
				WHEN $1 = 'proxmox_recent_failed_tasks_24h' THEN COALESCE(ft.failed_tasks, 0)::float
				ELSE n.cpu_usage
			END DESC,
			n.last_seen_at DESC
			LIMIT 1`, metric).Scan(&nodeID); err == nil && nodeID != "" {
			return db.resolveProxmoxNodeLabel(ctx, nodeID)
		}
		return "Cluster Proxmox"
	case "proxmox_guest_cpu_percent", "proxmox_guest_memory_percent":
		var guestID string
		if err := db.conn.QueryRowContext(ctx, `
			SELECT g.id
			FROM proxmox_guests g
			LEFT JOIN LATERAL (
				SELECT gm.cpu_usage,
				       CASE WHEN gm.mem_total > 0 THEN gm.mem_used::float / gm.mem_total ELSE 0 END AS mem_ratio
				FROM proxmox_guest_metrics gm
				WHERE gm.guest_id = g.id
				ORDER BY gm.timestamp DESC
				LIMIT 1
			) m ON TRUE
			ORDER BY CASE WHEN $1 = 'proxmox_guest_memory_percent' THEN COALESCE(m.mem_ratio, 0) ELSE COALESCE(m.cpu_usage, 0) END DESC,
			         g.last_seen_at DESC
			LIMIT 1`, metric).Scan(&guestID); err == nil && guestID != "" {
			return db.resolveProxmoxGuestLabel(ctx, guestID)
		}
		return "Cluster Proxmox"
	case "proxmox_storage_percent":
		var storageID string
		if err := db.conn.QueryRowContext(ctx, `
			SELECT id
			FROM proxmox_storages
			WHERE total > 0 AND enabled = TRUE AND active = TRUE
			ORDER BY (used::float / NULLIF(total, 0)) DESC, last_seen_at DESC
			LIMIT 1`).Scan(&storageID); err == nil && storageID != "" {
			return db.resolveProxmoxStorageLabel(ctx, storageID)
		}
		return "Cluster Proxmox"
	case "proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		var diskID string
		if err := db.conn.QueryRowContext(ctx, `
			SELECT id
			FROM proxmox_disks
			WHERE wearout >= 0
			ORDER BY CASE WHEN $1 = 'proxmox_disk_failed_count' THEN CASE WHEN health = 'FAILED' THEN 1 ELSE 0 END ELSE (100 - wearout) END DESC,
			         last_seen_at DESC
			LIMIT 1`, metric).Scan(&diskID); err == nil && diskID != "" {
			return db.resolveProxmoxDiskLabel(ctx, diskID)
		}
		return "Cluster Proxmox"
	default:
		return "Cluster Proxmox"
	}
}
