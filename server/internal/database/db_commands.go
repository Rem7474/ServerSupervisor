package database

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== Remote Commands ==========

// newUUID generates a UUID v4 using crypto/rand — no external dependency.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (db *DB) CreateRemoteCommand(hostID, module, action, target, payload, triggeredBy string, auditLogID *int64) (*models.RemoteCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	if payload == "" {
		payload = "{}"
	}
	id := newUUID()
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`INSERT INTO remote_commands (id, host_id, module, action, target, payload, triggered_by, audit_log_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at`,
		id, hostID, module, action, target, payload, triggeredBy, auditLogID,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	return &cmd, nil
}

// ClaimPendingRemoteCommands atomically claims all pending commands for a host
// and marks them as running before returning them. This enforces exactly-once
// delivery semantics across report polling cycles.
func (db *DB) ClaimPendingRemoteCommands(hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.Query(
		`WITH claimed AS (
SELECT id
FROM remote_commands
WHERE host_id = $1 AND status = 'pending'
ORDER BY created_at ASC
FOR UPDATE SKIP LOCKED
)
UPDATE remote_commands rc
SET status = 'running',
    started_at = COALESCE(rc.started_at, NOW())
FROM claimed
WHERE rc.id = claimed.id
RETURNING rc.id, rc.module, rc.action, rc.target, rc.payload`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cmds []models.PendingCommand
	for rows.Next() {
		var c models.PendingCommand
		if err := rows.Scan(&c.ID, &c.Module, &c.Action, &c.Target, &c.Payload); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func (db *DB) UpdateRemoteCommandStatus(id, status, output string) error {
	switch status {
	case "running":
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, started_at = NOW() WHERE id = $2`,
			status, id)
		return err
	default:
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`,
			status, output, id)
		return err
	}
}

func (db *DB) GetRemoteCommandByID(id string) (*models.RemoteCommand, error) {
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	var scheduledTaskID sql.NullString
	err := db.conn.QueryRow(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at, scheduled_task_id
		 FROM remote_commands WHERE id = $1`, id,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt, &scheduledTaskID)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	if scheduledTaskID.Valid {
		cmd.ScheduledTaskID = &scheduledTaskID.String
	}
	return &cmd, nil
}

// LinkCommandToScheduledTask associates a remote command with the scheduled task that triggered it.
func (db *DB) LinkCommandToScheduledTask(commandID, taskID string) error {
	_, err := db.conn.Exec(
		`UPDATE remote_commands SET scheduled_task_id = $1 WHERE id = $2`,
		taskID, commandID)
	return err
}

func (db *DB) GetRemoteCommandsByHostAndModule(hostID, module string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 AND module = $2 ORDER BY created_at DESC LIMIT $3`,
		hostID, module, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cmds []models.RemoteCommand
	for rows.Next() {
		var cmd models.RemoteCommand
		var startedAt, endedAt sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
			&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt); err != nil {
			continue
		}
		if startedAt.Valid {
			cmd.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			cmd.EndedAt = &endedAt.Time
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// GetRecentCommandsByHost returns the most recent remote commands for a host across all modules.
func (db *DB) GetRecentCommandsByHost(hostID string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 ORDER BY created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cmds []models.RemoteCommand
	for rows.Next() {
		var cmd models.RemoteCommand
		var startedAt, endedAt sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
			&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt); err != nil {
			continue
		}
		if startedAt.Valid {
			cmd.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			cmd.EndedAt = &endedAt.Time
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

// RemoteCommandWithHost embeds RemoteCommand and adds a resolved host name for display.
type RemoteCommandWithHost struct {
	models.RemoteCommand
	HostName string `json:"host_name"`
}

// GetAllRemoteCommands returns paginated remote commands for all hosts, joined with host name.
func (db *DB) GetAllRemoteCommands(limit, offset int) ([]RemoteCommandWithHost, error) {
	rows, err := db.conn.Query(`
		SELECT rc.id, rc.host_id, rc.module, rc.action, rc.target, rc.payload,
		       rc.status, rc.output, rc.triggered_by, rc.audit_log_id,
		       rc.created_at, rc.started_at, rc.ended_at,
		       COALESCE(h.name, rc.host_id) AS host_name
		FROM remote_commands rc
		LEFT JOIN hosts h ON h.id = rc.host_id
		ORDER BY rc.created_at DESC
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []RemoteCommandWithHost
	for rows.Next() {
		var c RemoteCommandWithHost
		var startedAt, endedAt sql.NullTime
		var auditLogID sql.NullInt64
		if err := rows.Scan(
			&c.ID, &c.HostID, &c.Module, &c.Action, &c.Target, &c.Payload,
			&c.Status, &c.Output, &c.TriggeredBy, &auditLogID,
			&c.CreatedAt, &startedAt, &endedAt,
			&c.HostName,
		); err != nil {
			continue
		}
		if auditLogID.Valid {
			c.AuditLogID = &auditLogID.Int64
		}
		if startedAt.Valid {
			c.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			c.EndedAt = &endedAt.Time
		}
		result = append(result, c)
	}
	return result, nil
}

// CreateCompletedRemoteCommand inserts a remote command that has already finished
// (e.g. a command the agent ran autonomously at startup). started_at and ended_at
// are both set to now, bypassing the usual pending→running→completed lifecycle.
func (db *DB) CreateCompletedRemoteCommand(hostID, module, action, target, output, triggeredBy string, status string, auditLogID *int64) error {
	id := newUUID()
	_, err := db.conn.Exec(`
		INSERT INTO remote_commands
		  (id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, started_at, ended_at)
		VALUES ($1,$2,$3,$4,$5,'{}', $6,$7,$8,$9, NOW(), NOW())`,
		id, hostID, module, action, target, status, output, triggeredBy, auditLogID,
	)
	return err
}

// CountAllRemoteCommands returns the total number of remote commands.
func (db *DB) CountAllRemoteCommands() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM remote_commands`).Scan(&count)
	return count, err
}

// GetRecentNotifications returns the latest alert incidents with enriched metadata
// for WebSocket browser notification delivery.
func (db *DB) GetRecentNotifications(limit int) ([]models.NotificationItem, error) {
	rows, err := db.conn.Query(
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
		db.enrichNotificationSource(&item)
		items = append(items, item)
	}
	return items, nil
}

func (db *DB) enrichNotificationSource(item *models.NotificationItem) {
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
		if label := db.resolveProxmoxNodeLabel(rawID); label != "" {
			item.SourceLabel = label
			return
		}
	case "guest":
		if label := db.resolveProxmoxGuestLabel(rawID); label != "" {
			item.SourceLabel = label
			return
		}
	case "storage":
		if label := db.resolveProxmoxStorageLabel(rawID); label != "" {
			item.SourceLabel = label
			return
		}
	case "disk":
		if label := db.resolveProxmoxDiskLabel(rawID); label != "" {
			item.SourceLabel = label
			return
		}
	case "connection":
		if label := db.resolveProxmoxConnectionLabel(rawID); label != "" {
			item.SourceLabel = label
			return
		}
	case "global":
		if label := db.resolveProxmoxGlobalLikelySource(item.Metric); label != "" {
			item.SourceLabel = label + " (source actuelle)"
			return
		}
	}

	if item.HostName == "" {
		item.SourceLabel = "Proxmox"
	} else {
		item.SourceLabel = item.HostName
	}
}

func (db *DB) resolveProxmoxConnectionLabel(connectionID string) string {
	if connectionID == "" {
		return "Proxmox cluster"
	}
	var name string
	if err := db.conn.QueryRow(`SELECT name FROM proxmox_connections WHERE id = $1`, connectionID).Scan(&name); err != nil {
		return "Connexion " + connectionID
	}
	return "Connexion " + name
}

func (db *DB) resolveProxmoxNodeLabel(nodeID string) string {
	if nodeID == "" {
		return "Noeud Proxmox"
	}
	var connName, nodeName string
	err := db.conn.QueryRow(`
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

func (db *DB) resolveProxmoxGuestLabel(guestID string) string {
	if guestID == "" {
		return "VM/LXC Proxmox"
	}
	var connName, nodeName, guestName, guestType string
	var vmid int
	err := db.conn.QueryRow(`
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

func (db *DB) resolveProxmoxStorageLabel(storageID string) string {
	if storageID == "" {
		return "Stockage Proxmox"
	}
	var connName, nodeName, storageName string
	err := db.conn.QueryRow(`
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

func (db *DB) resolveProxmoxDiskLabel(diskID string) string {
	if diskID == "" {
		return "Disque Proxmox"
	}
	var connName, nodeName, devPath, model string
	err := db.conn.QueryRow(`
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

func (db *DB) resolveProxmoxGlobalLikelySource(metric string) string {
	switch metric {
	case "proxmox_node_cpu_percent", "proxmox_node_memory_percent", "proxmox_node_cpu_temperature", "proxmox_node_fan_rpm", "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h":
		var nodeID string
		if err := db.conn.QueryRow(`
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
			return db.resolveProxmoxNodeLabel(nodeID)
		}
		return "Cluster Proxmox"
	case "proxmox_guest_cpu_percent", "proxmox_guest_memory_percent":
		var guestID string
		if err := db.conn.QueryRow(`
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
			return db.resolveProxmoxGuestLabel(guestID)
		}
		return "Cluster Proxmox"
	case "proxmox_storage_percent":
		var storageID string
		if err := db.conn.QueryRow(`
			SELECT id
			FROM proxmox_storages
			WHERE total > 0 AND enabled = TRUE AND active = TRUE
			ORDER BY (used::float / NULLIF(total, 0)) DESC, last_seen_at DESC
			LIMIT 1`).Scan(&storageID); err == nil && storageID != "" {
			return db.resolveProxmoxStorageLabel(storageID)
		}
		return "Cluster Proxmox"
	case "proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		var diskID string
		if err := db.conn.QueryRow(`
			SELECT id
			FROM proxmox_disks
			WHERE wearout >= 0
			ORDER BY CASE WHEN $1 = 'proxmox_disk_failed_count' THEN CASE WHEN health = 'FAILED' THEN 1 ELSE 0 END ELSE (100 - wearout) END DESC,
			         last_seen_at DESC
			LIMIT 1`, metric).Scan(&diskID); err == nil && diskID != "" {
			return db.resolveProxmoxDiskLabel(diskID)
		}
		return "Cluster Proxmox"
	default:
		return "Cluster Proxmox"
	}
}

// CleanupStalledCommands marks old pending/running commands as failed and closes linked audit logs
// and linked scheduled tasks.
func (db *DB) CleanupStalledCommands(timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $1
		RETURNING audit_log_id, scheduled_task_id`,
		timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup stalled commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var auditIDs []int64
	var taskIDs []string
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		var scheduledTaskID sql.NullString
		if err := rows.Scan(&auditLogID, &scheduledTaskID); err == nil {
			if auditLogID.Valid {
				auditIDs = append(auditIDs, auditLogID.Int64)
			}
			if scheduledTaskID.Valid {
				taskIDs = append(taskIDs, scheduledTaskID.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("Cleaned up %d stalled remote commands", count)
		if len(auditIDs) > 0 {
			if _, err := db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs)); err != nil {
				log.Printf("Warning: failed to update %d audit logs during cleanup: %v", len(auditIDs), err)
			}
		}
		if len(taskIDs) > 0 {
			if _, err := db.conn.Exec(`
				UPDATE scheduled_tasks SET last_run_status = 'failed', last_run_at = NOW()
				WHERE id = ANY($1)`,
				pq.Array(taskIDs)); err != nil {
				log.Printf("Warning: failed to update %d scheduled tasks during cleanup: %v", len(taskIDs), err)
			}
		}
	}
	return nil
}

// CleanupHostStalledCommands marks old pending/running commands for a specific host as failed.
func (db *DB) CleanupHostStalledCommands(hostID string, timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE host_id = $1
		  AND status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $2
		RETURNING audit_log_id, scheduled_task_id`,
		hostID, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup host stalled commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var auditIDs []int64
	var taskIDs []string
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		var scheduledTaskID sql.NullString
		if err := rows.Scan(&auditLogID, &scheduledTaskID); err == nil {
			if auditLogID.Valid {
				auditIDs = append(auditIDs, auditLogID.Int64)
			}
			if scheduledTaskID.Valid {
				taskIDs = append(taskIDs, scheduledTaskID.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		safeHostID := strings.ReplaceAll(hostID, "\n", "")
		safeHostID = strings.ReplaceAll(safeHostID, "\r", "")
		log.Printf("Cleaned up %d stalled commands for host %s", count, safeHostID)
		if len(auditIDs) > 0 {
			if _, err := db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs)); err != nil {
				log.Printf("Warning: failed to update %d audit logs for host %s during cleanup: %v", len(auditIDs), safeHostID, err)
			}
		}
		if len(taskIDs) > 0 {
			if _, err := db.conn.Exec(`
				UPDATE scheduled_tasks SET last_run_status = 'failed', last_run_at = NOW()
				WHERE id = ANY($1)`,
				pq.Array(taskIDs)); err != nil {
				log.Printf("Warning: failed to update %d scheduled tasks for host %s during cleanup: %v", len(taskIDs), safeHostID, err)
			}
		}
	}
	return nil
}
