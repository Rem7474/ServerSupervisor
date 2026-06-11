package alerts

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

// NotificationPusher broadcasts a real-time alert event to connected frontend clients.
// The api.NotificationHub implements this interface; pass nil to skip push.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

// CurrentIncidentValue returns the live metric value for an open incident,
// reconstructing the evaluation target the same way the engine does (real agent
// host, or synthetic Proxmox/synthetic target keyed by the incident's host_id).
func CurrentIncidentValue(ctx context.Context, db *database.DB, rule models.AlertRule, hostID string) (float64, bool) {
	var host models.Host
	if strings.HasPrefix(hostID, "proxmox:") || strings.HasPrefix(hostID, "synthetic:") ||
		strings.HasPrefix(hostID, "docker:") {
		host = models.Host{ID: hostID, Status: "online", LastSeen: time.Now()}
	} else {
		h, err := db.GetHost(ctx, hostID)
		if err != nil || h == nil {
			return 0, false
		}
		host = *h
	}
	return GetMetricValue(ctx, db, host, rule)
}

// ResolveStaleIncidentsForRule immediately resolves any open incidents for a rule
// that are no longer active under its current thresholds. Called after a rule
// update so stuck incidents don't wait for the next engine tick.
//
// Unlike the periodic engine, this function fetches the actual current metric
// value (not inc.Value, which is stale once the value drops below the trigger
// and the engine stops updating it).
func ResolveStaleIncidentsForRule(ctx context.Context, db *database.DB, rule models.AlertRule) {
	incidents, err := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil || len(incidents) == 0 {
		return
	}

	// Build a host map so status_offline can read the real host status.
	hostMap := map[string]models.Host{}
	if hosts, err := db.GetAllHosts(ctx); err == nil {
		for _, h := range hosts {
			hostMap[h.ID] = h
		}
	}

	for _, inc := range incidents {
		host, ok := hostMap[inc.HostID]
		if !ok {
			// Synthetic host (Proxmox / synthetic metrics) — construct a minimal record.
			host = models.Host{ID: inc.HostID, Status: "online", LastSeen: time.Now()}
		}

		// Fetch the actual current metric value, same as the engine does.
		currentValue, ok := GetMetricValue(ctx, db, host, rule)
		if !ok {
			continue
		}

		// Still firing with the new thresholds → leave open.
		if DetermineSeverity(rule, host, currentValue) != SeverityNone {
			continue
		}
		// Resolution conditions met → close immediately.
		if ShouldResolveAlertSeverity(rule, host, currentValue, AlertSeverity(inc.Severity)) {
			if err := db.ResolveAlertIncident(ctx, inc.ID); err == nil {
				slog.InfoContext(ctx, "alerts: stale incident resolved after rule update",
					slog.Int64("incident_id", inc.ID), slog.Int64("rule_id", rule.ID))
			}
		}
	}
}

func EvaluateAlerts(ctx context.Context, db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, pusher NotificationPusher) {
	rules, err := db.GetAlertRules(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to fetch rules", slog.Any("err", err))
		return
	}
	if len(rules) == 0 {
		return
	}

	hosts, err := db.GetAllHosts(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to fetch hosts", slog.Any("err", err))
		return
	}

	n := notify.New()

	for _, rule := range rules {
		if !rule.Enabled {
			resolvedCount, err := db.ResolveOpenAlertIncidentsByRule(ctx, rule.ID)
			if err != nil {
				slog.ErrorContext(ctx, "alerts: failed to resolve open incidents for disabled rule", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
			} else if resolvedCount > 0 {
				slog.InfoContext(ctx, "alerts: disabled rule resolved open incidents", slog.Int64("rule_id", rule.ID), slog.Int64("resolved", resolvedCount))
				broadcastIncidentUpdate(pusher, "resolved", rule, "")
			}
			continue
		}

		ruleName := fmt.Sprintf("rule#%d(%s %s)", rule.ID, rule.Metric, rule.Operator)
		if rule.Name != nil && *rule.Name != "" {
			ruleName = fmt.Sprintf("rule#%d(%s)", rule.ID, *rule.Name)
		}

		// Build evaluation targets:
		// - agent metrics: real hosts
		// - Proxmox metrics: synthetic targets (scope-based or per-entity for global rules)
		hostsForRule := buildAlertEvaluationTargets(ctx, db, rule, hosts)
		evaluatedTargets := make(map[string]struct{}, len(hostsForRule))

		for _, host := range hostsForRule {
			evaluatedTargets[host.ID] = struct{}{}
			if hasHostID(rule) && !isProxmoxMetric(rule.Metric) && *rule.HostID != host.ID {
				continue
			}

			value, ok := GetMetricValue(ctx, db, host, rule)
			if !ok {
				continue
			}

			// Determine current severity based on rule and value
			currentSeveration := DetermineSeverity(rule, host, value)

			// Get any open incident (regardless of severity)
			inc, err := db.GetOpenAlertIncident(ctx, rule.ID, host.ID)
			if err != nil && err != sql.ErrNoRows {
				slog.ErrorContext(ctx, "alerts: failed to check incidents", slog.Any("err", err))
				continue
			}

			if currentSeveration != SeverityNone {
				// Alert is triggered at current severity level
				if err == sql.ErrNoRows || inc == nil {
					// No existing incident - create new one with current severity
					incID, err := db.CreateAlertIncident(ctx, rule.ID, host.ID, value, string(currentSeveration))
					if err != nil {
						slog.ErrorContext(ctx, "alerts: failed to create incident", slog.Any("err", err))
						continue
					}
					slog.InfoContext(ctx, "alerts: incident FIRED", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Float64("value", value), slog.String("severity", string(currentSeveration)), slog.Int64("incident_id", incID))
					details := fmt.Sprintf(`{"rule_id":%d,"metric":"%s","operator":"%s","value":%.4f,"severity":"%s"}`, rule.ID, rule.Metric, rule.Operator, value, currentSeveration)
					_, _ = db.CreateAuditLog(ctx, "alert-engine", "alert_fired", host.ID, "", details, "success")
					sendAlertNotifications(n, cfg, rule, host, value)
					triggerAlertCommand(ctx, dispatcher, db, rule, host)
					pushBrowserNotification(pusher, rule, host, value, incID)
					go pushWebNotifications(ctx, db, cfg, rule, host, value)
					broadcastIncidentUpdate(pusher, "fired", rule, host.ID)
				} else {
					// Keep incident context fresh so UI and resolution logic use current severity/value.
					severityChanged := AlertSeverity(inc.Severity) != currentSeveration
					valueChanged := inc.Value != value
					hostChanged := inc.HostID != host.ID
					if severityChanged || valueChanged || hostChanged {
						if err := db.UpdateAlertIncidentContext(ctx, inc.ID, host.ID, value, string(currentSeveration)); err != nil {
							slog.ErrorContext(ctx, "alerts: failed to update incident context", slog.Int64("incident_id", inc.ID), slog.Any("err", err))
						} else if severityChanged {
							slog.InfoContext(ctx, "alerts: incident UPDATED", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Float64("value", value), slog.String("severity_from", inc.Severity), slog.String("severity_to", string(currentSeveration)), slog.Int64("incident_id", inc.ID))
						}
					}
				}
			} else if inc != nil {
				// No alert triggered - resolve if one exists
				if ShouldResolveAlertSeverity(rule, host, value, AlertSeverity(inc.Severity)) {
					_ = db.ResolveAlertIncident(ctx, inc.ID)
					slog.InfoContext(ctx, "alerts: incident resolved", slog.String("rule", ruleName), slog.String("host", host.Name), slog.String("severity", inc.Severity), slog.Int64("incident_id", inc.ID))
					details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d,"severity":"%s"}`, rule.ID, inc.ID, inc.Severity)
					_, _ = db.CreateAuditLog(ctx, "alert-engine", "alert_resolved", host.ID, "", details, "success")
					broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
				}
			}
		}

		if isProxmoxGlobalScope(rule) {
			resolveStaleGlobalProxmoxIncidents(ctx, db, rule, evaluatedTargets)
		}
	}
}

func isProxmoxGlobalScope(rule models.AlertRule) bool {
	if !isProxmoxMetric(rule.Metric) {
		return false
	}
	scope := proxmoxScopeFromRule(rule)
	return scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global"
}

// isProxmoxMetric detects if a metric belongs to the Proxmox subsystem.
func isProxmoxMetric(metric string) bool {
	switch metric {
	case "proxmox_storage_percent",
		"proxmox_node_cpu_percent",
		"proxmox_node_memory_percent",
		"proxmox_node_cpu_temperature",
		"proxmox_node_fan_rpm",
		"proxmox_guest_cpu_percent",
		"proxmox_guest_memory_percent",
		"proxmox_node_pending_updates",
		"proxmox_recent_failed_tasks_24h",
		"proxmox_auth_failures_recent",
		"proxmox_disk_failed_count",
		"proxmox_disk_min_wearout_percent":
		return true
	default:
		return false
	}
}

// isSyntheticMetric detects if a metric belongs to the synthetic monitoring
// subsystem (uptime probes, SSL certificates). These metrics are global and
// evaluated once per rule, not per host.
func isSyntheticMetric(metric string) bool {
	switch metric {
	case "uptime_down_count", "ssl_min_days_remaining":
		return true
	default:
		return false
	}
}

// hasHostID checks if a rule explicitly filters by host ID.
func hasHostID(rule models.AlertRule) bool {
	return rule.HostID != nil && *rule.HostID != ""
}

func proxmoxScopeFromRule(rule models.AlertRule) *models.ProxmoxMetricScope {
	return rule.ProxmoxScope
}

// proxmoxScopeKey generates a unique identifier for a Proxmox alert incident
// based on scope mode to avoid duplicate incidents per scope.
func proxmoxScopeKey(scope *models.ProxmoxMetricScope) string {
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return "proxmox:global"
	}
	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID != "" {
			return fmt.Sprintf("proxmox:connection:%s", scope.ConnectionID)
		}
	case "node":
		if scope.NodeID != "" {
			return fmt.Sprintf("proxmox:node:%s", scope.NodeID)
		}
	case "storage":
		if scope.StorageID != "" {
			return fmt.Sprintf("proxmox:storage:%s", scope.StorageID)
		}
	case "guest":
		if scope.GuestID != "" {
			return fmt.Sprintf("proxmox:guest:%s", scope.GuestID)
		}
	case "disk":
		if scope.DiskID != "" {
			return fmt.Sprintf("proxmox:disk:%s", scope.DiskID)
		}
	}
	return "proxmox:global"
}

// proxmoxScopeLabel formats a human-readable description of a Proxmox scope for incident messages.
func proxmoxScopeLabel(scope *models.ProxmoxMetricScope) string {
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return "Proxmox global"
	}
	switch scope.ScopeMode {
	case "connection":
		return fmt.Sprintf("Proxmox connexion %s", scope.ConnectionID)
	case "node":
		return fmt.Sprintf("Proxmox noeud %s", scope.NodeID)
	case "storage":
		return fmt.Sprintf("Proxmox stockage %s", scope.StorageID)
	case "guest":
		return fmt.Sprintf("Proxmox VM/LXC %s", scope.GuestID)
	case "disk":
		return fmt.Sprintf("Proxmox disque %s", scope.DiskID)
	}
	return "Proxmox global"
}

// buildAlertEvaluationTargets creates the list of hosts/targets to evaluate for a rule.
// For agent metrics, returns the provided hosts. For Proxmox metrics, returns a single
// synthetic host record with ID from proxmoxScopeKey() to deduplicate incidents per scope.
// For Docker metrics, returns synthetic targets per container or per host aggregate.
func buildAlertEvaluationTargets(ctx context.Context, db *database.DB, rule models.AlertRule, hosts []models.Host) []models.Host {
	if isSyntheticMetric(rule.Metric) {
		// Synthetic metrics are global — evaluate once with a single synthetic target so
		// the engine creates exactly one incident per rule on fire.
		return []models.Host{{
			ID:       "synthetic:" + rule.Metric,
			Name:     "Monitoring synthétique",
			Status:   "online",
			LastSeen: time.Now(),
		}}
	}
	if isDockerMetric(rule.Metric) {
		return buildDockerEvaluationTargets(ctx, db, rule)
	}
	if !isProxmoxMetric(rule.Metric) {
		// For agent metrics, filter by HostID if set
		if hasHostID(rule) {
			for _, h := range hosts {
				if h.ID == *rule.HostID {
					return []models.Host{h}
				}
			}
			return []models.Host{}
		}
		return hosts
	}

	if isProxmoxGlobalScope(rule) {
		if targets := buildGlobalProxmoxEntityTargets(ctx, db, rule); len(targets) > 0 {
			return targets
		}
	}

	// For Proxmox metrics, create a synthetic host record with scope-based ID
	// This ensures one incident per Proxmox scope, not per agent host
	syntheticID := proxmoxScopeKey(proxmoxScopeFromRule(rule))
	syntheticLabel := proxmoxScopeLabel(proxmoxScopeFromRule(rule))
	return []models.Host{
		{
			ID:       syntheticID,
			Name:     syntheticLabel,
			Status:   "online",
			LastSeen: time.Now(),
		},
	}
}

func isDockerMetric(metric string) bool {
	return models.IsDockerMetric(metric)
}

// BuildDockerTestTargets is the exported entry point for the test-run handler.
func BuildDockerTestTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	return buildDockerEvaluationTargets(ctx, db, rule)
}

// buildDockerEvaluationTargets returns synthetic targets for Docker metrics.
// For docker_container_not_running with scope=host: one target per container on the host.
// For docker_container_not_running with scope=container: one target for the specific container.
// For docker_container_running_count: one aggregate target for the host.
func buildDockerEvaluationTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	scope := rule.DockerScope
	if scope == nil || scope.HostID == "" {
		return nil
	}

	switch rule.Metric {
	case "docker_container_state":
		switch scope.ScopeMode {
		case "host":
			containers, err := db.ListDockerContainersForAlerts(ctx, scope.HostID)
			if err != nil {
				return nil
			}
			targets := make([]models.Host, 0, len(containers))
			for _, c := range containers {
				targets = append(targets, models.Host{
					ID:       "docker:container:" + c.ID,
					Name:     c.Name + " (" + c.Image + ":" + c.ImageTag + ")",
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
			return targets
		case "container":
			c, err := db.GetDockerContainerByID(ctx, scope.ContainerID)
			if err != nil || c == nil {
				return nil
			}
			return []models.Host{{
				ID:       "docker:container:" + c.ID,
				Name:     c.Name + " (" + c.Image + ":" + c.ImageTag + ")",
				Status:   "online",
				LastSeen: time.Now(),
			}}
		}
	case "docker_compose_degraded_services":
		return []models.Host{{
			ID:       "docker:compose:" + scope.HostID + ":" + scope.ProjectName,
			Name:     "Compose " + scope.ProjectName,
			Status:   "online",
			LastSeen: time.Now(),
		}}
	}
	return nil
}

func buildGlobalProxmoxEntityTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	switch rule.Metric {
	case "proxmox_node_cpu_percent", "proxmox_node_memory_percent", "proxmox_node_cpu_temperature", "proxmox_node_fan_rpm", "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_auth_failures_recent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0, len(nodes))
		for _, n := range nodes {
			targets = append(targets, models.Host{
				ID:       "proxmox:node:" + n.ID,
				Name:     fmt.Sprintf("Proxmox noeud %s", n.NodeName),
				Status:   "online",
				LastSeen: time.Now(),
			})
		}
		return targets
	case "proxmox_guest_cpu_percent", "proxmox_guest_memory_percent":
		guests, err := db.ListProxmoxGuests(ctx, "", "", "")
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0, len(guests))
		for _, g := range guests {
			name := g.Name
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("VM/LXC %d", g.VMID)
			}
			targets = append(targets, models.Host{
				ID:       "proxmox:guest:" + g.ID,
				Name:     fmt.Sprintf("Proxmox VM/LXC %s", name),
				Status:   "online",
				LastSeen: time.Now(),
			})
		}
		return targets
	case "proxmox_storage_percent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0)
		for _, n := range nodes {
			storages, err := db.ListProxmoxStoragesByNode(ctx, n.ConnectionID, n.NodeName)
			if err != nil {
				continue
			}
			for _, s := range storages {
				targets = append(targets, models.Host{
					ID:       "proxmox:storage:" + s.ID,
					Name:     fmt.Sprintf("Proxmox stockage %s", s.StorageName),
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
		}
		return targets
	case "proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0)
		for _, n := range nodes {
			disks, err := db.ListProxmoxDisksByNode(ctx, n.ConnectionID, n.NodeName)
			if err != nil {
				continue
			}
			for _, d := range disks {
				targets = append(targets, models.Host{
					ID:       "proxmox:disk:" + d.ID,
					Name:     fmt.Sprintf("Proxmox disque %s", d.DevPath),
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
		}
		return targets
	default:
		return nil
	}
}

func proxmoxScopedRuleForSyntheticTarget(rule models.AlertRule, targetID string) (models.AlertRule, bool) {
	if !isProxmoxGlobalScope(rule) {
		return rule, false
	}

	parts := strings.SplitN(targetID, ":", 3)
	if len(parts) != 3 || parts[0] != "proxmox" || parts[2] == "" {
		return rule, false
	}

	scoped := rule
	scope := &models.ProxmoxMetricScope{}
	entityType := parts[1]
	entityID := parts[2]

	switch entityType {
	case "node":
		scope.ScopeMode = "node"
		scope.NodeID = entityID
	case "guest":
		scope.ScopeMode = "guest"
		scope.GuestID = entityID
	case "storage":
		scope.ScopeMode = "storage"
		scope.StorageID = entityID
	case "disk":
		scope.ScopeMode = "disk"
		scope.DiskID = entityID
	default:
		return rule, false
	}

	scoped.ProxmoxScope = scope
	return scoped, true
}

func resolveStaleGlobalProxmoxIncidents(ctx context.Context, db *database.DB, rule models.AlertRule, evaluatedTargets map[string]struct{}) {
	openIncidents, err := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to list open incidents for stale cleanup", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
		return
	}

	for _, inc := range openIncidents {
		if !strings.HasPrefix(inc.HostID, "proxmox:") {
			continue
		}
		if _, ok := evaluatedTargets[inc.HostID]; ok {
			continue
		}
		if err := db.ResolveAlertIncident(ctx, inc.ID); err != nil {
			slog.ErrorContext(ctx, "alerts: failed to resolve stale incident", slog.Int64("incident_id", inc.ID), slog.Int64("rule_id", rule.ID), slog.Any("err", err))
		}
	}
}
