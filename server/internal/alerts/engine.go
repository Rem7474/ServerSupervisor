package alerts

import (
	"context"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// NotificationPusher broadcasts a real-time alert event to connected frontend clients.
// The api.NotificationHub implements this interface; pass nil to skip push.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

func EvaluateAlerts(ctx context.Context, db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, pusher NotificationPusher) {
	rules, err := db.GetAlertRules(ctx)
	if err != nil {
		log.Printf("Alerts: failed to fetch rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	hosts, err := db.GetAllHosts(ctx)
	if err != nil {
		log.Printf("Alerts: failed to fetch hosts: %v", err)
		return
	}

	n := notify.New()

	for _, rule := range rules {
		if !rule.Enabled {
			resolvedCount, err := db.ResolveOpenAlertIncidentsByRule(ctx, rule.ID)
			if err != nil {
				log.Printf("Alerts: failed to resolve open incidents for disabled rule#%d: %v", rule.ID, err)
			} else if resolvedCount > 0 {
				log.Printf("Alerts: disabled rule#%d resolved %d open incident(s)", rule.ID, resolvedCount)
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
				log.Printf("Alerts: failed to check incidents: %v", err)
				continue
			}

			if currentSeveration != SeverityNone {
				// Alert is triggered at current severity level
				if err == sql.ErrNoRows || inc == nil {
					// No existing incident - create new one with current severity
					incID, err := db.CreateAlertIncident(ctx, rule.ID, host.ID, value, string(currentSeveration))
					if err != nil {
						log.Printf("Alerts: failed to create incident: %v", err)
						continue
					}
					log.Printf("Alerts: FIRED %s host=%s value=%.2f severity=%s → incident#%d created", ruleName, host.Name, value, currentSeveration, incID)
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
							log.Printf("Alerts: failed to update incident context for incident#%d: %v", inc.ID, err)
						} else if severityChanged {
							log.Printf("Alerts: UPDATED %s host=%s value=%.2f severity %s→%s incident#%d", ruleName, host.Name, value, inc.Severity, currentSeveration, inc.ID)
						}
					}
				}
			} else if inc != nil {
				// No alert triggered - resolve if one exists
				if ShouldResolveAlertSeverity(rule, host, value, AlertSeverity(inc.Severity)) {
					_ = db.ResolveAlertIncident(ctx, inc.ID)
					log.Printf("Alerts: %s host=%s severity=%s — resolved incident#%d", ruleName, host.Name, inc.Severity, inc.ID)
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
		log.Printf("Alerts: failed to list open incidents for stale cleanup rule#%d: %v", rule.ID, err)
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
			log.Printf("Alerts: failed to resolve stale incident#%d for rule#%d: %v", inc.ID, rule.ID, err)
		}
	}
}

// GetMetricValue retrieves the current value of a metric for a host according to a rule.
func GetMetricValue(ctx context.Context, db *database.DB, host models.Host, rule models.AlertRule) (float64, bool) {
	if scopedRule, ok := proxmoxScopedRuleForSyntheticTarget(rule, host.ID); ok {
		rule = scopedRule
	}

	now := time.Now()
	duration := time.Duration(rule.DurationSeconds) * time.Second

	switch rule.Metric {
	case "status_offline":
		if rule.DurationSeconds > 0 && now.Sub(host.LastSeen) < duration {
			return 0, false
		}
		if host.Status == "offline" {
			return 1, true
		}
		return 0, true
	case "heartbeat_timeout":
		// Returns seconds since the agent last reported. Threshold is the silence duration in seconds.
		return now.Sub(host.LastSeen).Seconds(), true
	case "cpu", "memory", "disk", "load", "cpu_temperature":
		// When a host is explicitly linked to a Proxmox guest in confirmed+auto mode,
		// CPU/RAM alerts must use guest metrics collected from Proxmox (hypervisor view).
		if rule.Metric == "cpu" || rule.Metric == "memory" {
			if link, err := db.GetProxmoxGuestLinkByHost(ctx, host.ID); err == nil && link != nil && link.Status == "confirmed" && link.MetricsSource == "auto" {
				cpuPct, memPct, ts, err := db.GetLatestProxmoxGuestMetricPercent(ctx, link.GuestID)
				if err == nil {
					if rule.DurationSeconds > 0 && now.Sub(ts) > duration {
						return 0, false
					}
					if rule.Metric == "cpu" {
						return cpuPct, true
					}
					return memPct, true
				}
			}
		}

		metrics, err := db.GetLatestMetrics(ctx, host.ID)
		if err != nil {
			return 0, false
		}
		if rule.DurationSeconds > 0 && now.Sub(metrics.Timestamp) > duration {
			return 0, false
		}
		switch rule.Metric {
		case "cpu":
			return metrics.CPUUsagePercent, true
		case "cpu_temperature":
			temp, ok := db.GetEffectiveHostCPUTemperature(ctx, host.ID, metrics.CPUTemperature)
			if !ok || temp <= 0 {
				return 0, false
			}
			return temp, true
		case "memory":
			return metrics.MemoryPercent, true
		case "disk":
			maxDisk := 0.0
			for _, d := range metrics.Disks {
				if d.UsedPercent > maxDisk {
					maxDisk = d.UsedPercent
				}
			}
			return maxDisk, true
		case "load":
			return metrics.LoadAvg1, true
		}
		return 0, false
	case "disk_smart_status":
		// Returns 1 if any disk has FAILED SMART status, 0 otherwise.
		healthData, err := db.GetLatestDiskHealth(ctx, host.ID)
		if err != nil || len(healthData) == 0 {
			return 0, false
		}
		for _, h := range healthData {
			if h.SmartStatus == "FAILED" {
				return 1, true
			}
		}
		return 0, true
	case "disk_temperature":
		// Returns the maximum temperature (°C) across all disks for this host.
		healthData, err := db.GetLatestDiskHealth(ctx, host.ID)
		if err != nil || len(healthData) == 0 {
			return 0, false
		}
		maxTemp := 0.0
		for _, h := range healthData {
			if float64(h.Temperature) > maxTemp {
				maxTemp = float64(h.Temperature)
			}
		}
		return maxTemp, true
	case "proxmox_storage_percent":
		pct := resolveProxmoxStoragePercent(ctx, db, rule)
		return pct, true
	case "proxmox_node_cpu_percent":
		pct := resolveProxmoxNodeCPUPercent(ctx, db, rule)
		return pct, true
	case "proxmox_node_memory_percent":
		pct := resolveProxmoxNodeMemoryPercent(ctx, db, rule)
		return pct, true
	case "proxmox_node_cpu_temperature":
		return resolveProxmoxNodeCPUTemperature(ctx, db, rule), true
	case "proxmox_node_fan_rpm":
		return resolveProxmoxNodeFanRPM(ctx, db, rule), true
	case "proxmox_guest_cpu_percent":
		return resolveProxmoxGuestCPUPercent(ctx, db, rule), true
	case "proxmox_guest_memory_percent":
		return resolveProxmoxGuestMemoryPercent(ctx, db, rule), true
	case "proxmox_node_pending_updates":
		return resolveProxmoxNodePendingUpdates(ctx, db, rule), true
	case "proxmox_recent_failed_tasks_24h":
		return resolveProxmoxRecentFailedTasks24h(ctx, db, rule), true
	case "proxmox_auth_failures_recent":
		return resolveProxmoxAuthFailuresRecent(ctx, db, rule), true
	case "proxmox_disk_failed_count":
		return resolveProxmoxDiskFailedCount(ctx, db, rule), true
	case "proxmox_disk_min_wearout_percent":
		return resolveProxmoxDiskMinWearoutPercent(ctx, db, rule), true
	case "uptime_down_count":
		// Global: how many enabled uptime probes are currently DOWN.
		n, err := db.CountDownProbes(ctx)
		if err != nil {
			return 0, false
		}
		return float64(n), true
	case "ssl_min_days_remaining":
		// Global: smallest "days until expiration" across all enabled SSL certs.
		// Returns false (skip evaluation) when no cert has a known valid_to yet.
		days, ok, err := db.GetMinSSLDaysRemaining(ctx)
		if err != nil || !ok {
			return 0, false
		}
		return float64(days), true
	}
	return 0, false
}

func resolveProxmoxStoragePercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxStorageUsagePercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxStorageUsagePercent(ctx)
		}
		return db.GetMaxProxmoxStorageUsagePercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxStorageUsagePercent(ctx)
		}
		return db.GetMaxProxmoxStorageUsagePercentByNode(ctx, scope.NodeID)
	case "storage":
		if scope.StorageID == "" {
			return db.GetMaxProxmoxStorageUsagePercent(ctx)
		}
		return db.GetProxmoxStorageUsagePercentByStorage(ctx, scope.StorageID)
	default:
		return db.GetMaxProxmoxStorageUsagePercent(ctx)
	}
}

// resolveProxmoxNodeCPUPercent returns the CPU usage for a Proxmox node metric
// based on the scope defined in the rule (global, connection, or node).
func resolveProxmoxNodeCPUPercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeCPUUsagePercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeCPUUsagePercent(ctx)
		}
		return db.GetMaxProxmoxNodeCPUUsagePercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeCPUUsagePercent(ctx)
		}
		return db.GetProxmoxNodeCPUUsagePercentByNode(ctx, scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeCPUUsagePercent(ctx)
	}
}

// resolveProxmoxNodeMemoryPercent returns the memory usage for a Proxmox node metric
// based on the scope defined in the rule (global, connection, or node).
func resolveProxmoxNodeMemoryPercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeMemoryUsagePercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeMemoryUsagePercent(ctx)
		}
		return db.GetMaxProxmoxNodeMemoryUsagePercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeMemoryUsagePercent(ctx)
		}
		return db.GetProxmoxNodeMemoryUsagePercentByNode(ctx, scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeMemoryUsagePercent(ctx)
	}
}

func resolveProxmoxNodeCPUTemperature(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeCPUTemperature(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeCPUTemperature(ctx)
		}
		return db.GetMaxProxmoxNodeCPUTemperatureByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeCPUTemperature(ctx)
		}
		return db.GetProxmoxNodeCPUTemperatureByNode(ctx, scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeCPUTemperature(ctx)
	}
}

func resolveProxmoxNodeFanRPM(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeFanRPM(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeFanRPM(ctx)
		}
		return db.GetMaxProxmoxNodeFanRPMByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeFanRPM(ctx)
		}
		return db.GetProxmoxNodeFanRPMByNode(ctx, scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeFanRPM(ctx)
	}
}

func resolveProxmoxGuestCPUPercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
		}
		return db.GetMaxProxmoxGuestCPUUsagePercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
		}
		return db.GetMaxProxmoxGuestCPUUsagePercentByNode(ctx, scope.NodeID)
	case "guest":
		if scope.GuestID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
		}
		cpu, _, _, err := db.GetLatestProxmoxGuestMetricPercent(ctx, scope.GuestID)
		if err != nil {
			return 0
		}
		return cpu
	default:
		return db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
	}
}

func resolveProxmoxGuestMemoryPercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
		}
		return db.GetMaxProxmoxGuestMemoryUsagePercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
		}
		return db.GetMaxProxmoxGuestMemoryUsagePercentByNode(ctx, scope.NodeID)
	case "guest":
		if scope.GuestID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
		}
		_, mem, _, err := db.GetLatestProxmoxGuestMetricPercent(ctx, scope.GuestID)
		if err != nil {
			return 0
		}
		return mem
	default:
		return db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
	}
}

func resolveProxmoxNodePendingUpdates(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(db.GetMaxProxmoxNodePendingUpdates(ctx))
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(db.GetMaxProxmoxNodePendingUpdates(ctx))
		}
		return float64(db.GetMaxProxmoxNodePendingUpdatesByConnection(ctx, scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(db.GetMaxProxmoxNodePendingUpdates(ctx))
		}
		return float64(db.GetMaxProxmoxNodePendingUpdatesByNode(ctx, scope.NodeID))
	default:
		return float64(db.GetMaxProxmoxNodePendingUpdates(ctx))
	}
}

func resolveProxmoxRecentFailedTasks24h(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	since := time.Now().Add(-24 * time.Hour)
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		count, err := db.GetRecentFailedTaskCount(ctx, since)
		if err != nil {
			return 0
		}
		return float64(count)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			count, err := db.GetRecentFailedTaskCount(ctx, since)
			if err != nil {
				return 0
			}
			return float64(count)
		}
		count, err := db.GetRecentFailedTaskCountByConnection(ctx, scope.ConnectionID, since)
		if err != nil {
			return 0
		}
		return float64(count)
	case "node":
		if scope.NodeID == "" {
			count, err := db.GetRecentFailedTaskCount(ctx, since)
			if err != nil {
				return 0
			}
			return float64(count)
		}
		count, err := db.GetRecentFailedTaskCountByNodeID(ctx, scope.NodeID, since)
		if err != nil {
			return 0
		}
		return float64(count)
	default:
		count, err := db.GetRecentFailedTaskCount(ctx, since)
		if err != nil {
			return 0
		}
		return float64(count)
	}
}

func resolveProxmoxAuthFailuresRecent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	window := time.Duration(rule.DurationSeconds) * time.Second
	if window <= 0 {
		window = 10 * time.Minute
	}
	since := time.Now().Add(-window)

	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
		}
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
		}
		node, err := db.GetProxmoxNode(ctx, scope.NodeID)
		if err != nil || node == nil {
			return 0
		}
		return float64(countAuthFailuresForNode(ctx, db, *node, since))
	default:
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
	}
}

// FetchProxmoxAuthFailureLogs returns the syslog lines used for the proxmox_auth_failures_recent metric.
// The returned lines are already filtered by the requested duration window.
func FetchProxmoxAuthFailureLogs(ctx context.Context, db *database.DB, rule models.AlertRule) ([]string, time.Time) {
	window := time.Duration(rule.DurationSeconds) * time.Second
	if window <= 0 {
		window = 10 * time.Minute
	}
	since := time.Now().Add(-window)

	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
		}
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, scope.ConnectionID), since
	case "node":
		if scope.NodeID == "" {
			return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
		}
		node, err := db.GetProxmoxNode(ctx, scope.NodeID)
		if err != nil || node == nil {
			return []string{}, since
		}
		return collectAuthFailureLogsForNode(ctx, db, *node, since), since
	default:
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
	}
}

func countAuthFailuresAcrossNodes(ctx context.Context, db *database.DB, since time.Time, connectionID string) int {
	var nodes []models.ProxmoxNode
	var err error

	if strings.TrimSpace(connectionID) == "" {
		nodes, err = db.ListProxmoxNodes(ctx)
	} else {
		nodes, err = db.ListProxmoxNodesByConnection(ctx, connectionID)
	}
	if err != nil || len(nodes) == 0 {
		return 0
	}

	count := 0
	for _, node := range nodes {
		count += countAuthFailuresForNode(ctx, db, node, since)
	}
	return count
}

func collectAuthFailureLogsAcrossNodes(ctx context.Context, db *database.DB, since time.Time, connectionID string) []string {
	var nodes []models.ProxmoxNode
	var err error

	if strings.TrimSpace(connectionID) == "" {
		nodes, err = db.ListProxmoxNodes(ctx)
	} else {
		nodes, err = db.ListProxmoxNodesByConnection(ctx, connectionID)
	}
	if err != nil || len(nodes) == 0 {
		return []string{}
	}

	var out []string
	for _, node := range nodes {
		out = append(out, collectAuthFailureLogsForNode(ctx, db, node, since)...)
	}
	return out
}

func countAuthFailuresForNode(ctx context.Context, db *database.DB, node models.ProxmoxNode, since time.Time) int {
	conn, err := db.GetProxmoxConnectionByID(ctx, node.ConnectionID)
	if err != nil || conn == nil {
		return 0
	}
	secret, err := db.GetProxmoxTokenSecret(ctx, node.ConnectionID)
	if err != nil || secret == "" {
		return 0
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	services := []string{"pvedaemon", "pveproxy", "sshd"}
	limit := estimateAuthFailureLimit(since)

	count := 0
	for _, service := range services {
		lines, err := client.GetNodeSyslog(node.NodeName, limit, service)
		if err != nil {
			log.Printf("Alerts: syslog fetch failed [%s/%s %s]: %v", conn.Name, node.NodeName, service, err)
			continue
		}
		count += countAuthFailuresInLines(lines, since)
	}
	return count
}

func collectAuthFailureLogsForNode(ctx context.Context, db *database.DB, node models.ProxmoxNode, since time.Time) []string {
	conn, err := db.GetProxmoxConnectionByID(ctx, node.ConnectionID)
	if err != nil || conn == nil {
		return []string{}
	}
	secret, err := db.GetProxmoxTokenSecret(ctx, node.ConnectionID)
	if err != nil || secret == "" {
		return []string{}
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	services := []string{"pvedaemon", "pveproxy", "sshd"}
	limit := estimateAuthFailureLimit(since)

	var out []string
	for _, service := range services {
		lines, err := client.GetNodeSyslog(node.NodeName, limit, service)
		if err != nil {
			log.Printf("Alerts: syslog fetch failed [%s/%s %s]: %v", conn.Name, node.NodeName, service, err)
			continue
		}
		out = append(out, authFailureLogLines(lines, since, node.NodeName)...)
	}
	return out
}

func authFailureLogLines(lines []proxmoxclient.PVESyslogLine, since time.Time, nodeName string) []string {
	var out []string
	for _, line := range lines {
		if !isAuthFailureSyslogLine(line) {
			continue
		}
		ts, ok := syslogLineTime(line)
		if !ok || ts.Before(since) {
			continue
		}
		text := formatSyslogLineText(line)
		if text == "" {
			continue
		}
		if nodeName != "" {
			text = fmt.Sprintf("[%s] %s", nodeName, text)
		}
		out = append(out, text)
	}
	return out
}

func formatSyslogLineText(line proxmoxclient.PVESyslogLine) string {
	if strings.TrimSpace(line.T) != "" {
		return strings.TrimSpace(line.T)
	}
	if strings.TrimSpace(line.Msg) != "" {
		return strings.TrimSpace(line.Msg)
	}
	parts := []string{}
	if strings.TrimSpace(line.Tag) != "" {
		parts = append(parts, strings.TrimSpace(line.Tag))
	}
	if strings.TrimSpace(line.Level) != "" {
		parts = append(parts, strings.TrimSpace(line.Level))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func countAuthFailuresInLines(lines []proxmoxclient.PVESyslogLine, since time.Time) int {
	count := 0
	for _, line := range lines {
		if !isAuthFailureSyslogLine(line) {
			continue
		}
		ts, ok := syslogLineTime(line)
		if !ok {
			// Skip lines without a usable timestamp so duration filters remain meaningful.
			continue
		}
		if ts.Before(since) {
			continue
		}
		count++
	}
	return count
}

func isAuthFailureSyslogLine(line proxmoxclient.PVESyslogLine) bool {
	text := strings.ToLower(strings.TrimSpace(strings.Join([]string{line.T, line.Msg, line.Tag, line.Level}, " ")))
	if text == "" {
		return false
	}
	return strings.Contains(text, "authentication failure") ||
		strings.Contains(text, "failed password") ||
		strings.Contains(text, "invalid user") ||
		strings.Contains(text, "too many authentication failures") ||
		strings.Contains(text, "maximum authentication attempts exceeded")
}

func estimateAuthFailureLimit(since time.Time) int {
	window := time.Since(since)
	if window <= 0 {
		return 300
	}
	// Heuristic: assume up to ~60 lines/minute for auth-related services.
	limit := int(window.Minutes()*60) + 50
	if limit < 300 {
		return 300
	}
	if limit > 5000 {
		return 5000
	}
	return limit
}

func syslogLineTime(line proxmoxclient.PVESyslogLine) (time.Time, bool) {
	if line.Time > 0 {
		sec := line.Time
		if sec > 946_684_800 {
			ms := sec
			if ms < 1_000_000_000_000 {
				ms *= 1000
			}
			return time.Unix(0, ms*int64(time.Millisecond)).UTC(), true
		}
	}

	if strings.TrimSpace(line.T) == "" {
		return time.Time{}, false
	}

	stamp, ok := extractSyslogTimestamp(line.T)
	if !ok {
		return time.Time{}, false
	}

	// Parse syslog timestamps as local time (logs use system timezone, which is local).
	parsed, err := time.ParseInLocation("Jan 2 15:04:05", stamp, time.Local)
	if err != nil {
		return time.Time{}, false
	}

	now := time.Now()
	// Reconstruct with current year in local time.
	parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	if parsed.After(now.Add(24 * time.Hour)) {
		parsed = parsed.AddDate(-1, 0, 0)
	}
	return parsed, true
}

func extractSyslogTimestamp(text string) (string, bool) {
	// Expected prefix: "May 6 12:34:56"
	re := regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

func resolveProxmoxDiskFailedCount(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(db.GetProxmoxDiskFailedCount(ctx))
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(db.GetProxmoxDiskFailedCount(ctx))
		}
		return float64(db.GetProxmoxDiskFailedCountByConnection(ctx, scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(db.GetProxmoxDiskFailedCount(ctx))
		}
		return float64(db.GetProxmoxDiskFailedCountByNodeID(ctx, scope.NodeID))
	case "disk":
		if scope.DiskID == "" {
			return float64(db.GetProxmoxDiskFailedCount(ctx))
		}
		return float64(db.GetProxmoxDiskFailedCountByDiskID(ctx, scope.DiskID))
	default:
		return float64(db.GetProxmoxDiskFailedCount(ctx))
	}
}

func resolveProxmoxDiskMinWearoutPercent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetProxmoxDiskMinWearoutPercent(ctx)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetProxmoxDiskMinWearoutPercent(ctx)
		}
		return db.GetProxmoxDiskMinWearoutPercentByConnection(ctx, scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetProxmoxDiskMinWearoutPercent(ctx)
		}
		return db.GetProxmoxDiskMinWearoutPercentByNodeID(ctx, scope.NodeID)
	case "disk":
		if scope.DiskID == "" {
			return db.GetProxmoxDiskMinWearoutPercent(ctx)
		}
		return db.GetProxmoxDiskWearoutPercentByDiskID(ctx, scope.DiskID)
	default:
		return db.GetProxmoxDiskMinWearoutPercent(ctx)
	}
}

// MatchRule evaluates whether a rule condition is currently met for the given value.
// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityNone AlertSeverity = ""
	SeverityWarn AlertSeverity = "warn"
	SeverityCrit AlertSeverity = "crit"
)

// DetermineSeverity returns the highest severity level (crit > warn > none) triggered by the rule
func DetermineSeverity(rule models.AlertRule, host models.Host, value float64) AlertSeverity {
	if rule.Metric == "status_offline" {
		if host.Status == "offline" {
			return SeverityCrit
		}
		return SeverityNone
	}

	if rule.ThresholdCrit == nil && rule.ThresholdWarn == nil {
		return SeverityNone
	}

	// Check critical threshold first
	if rule.ThresholdCrit != nil && matchThreshold(rule.Operator, value, *rule.ThresholdCrit) {
		return SeverityCrit
	}

	// Check warning threshold
	if rule.ThresholdWarn != nil && matchThreshold(rule.Operator, value, *rule.ThresholdWarn) {
		return SeverityWarn
	}

	return SeverityNone
}

// matchThreshold is a helper that checks if value matches operator condition against threshold
func matchThreshold(operator string, value float64, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	default:
		return false
	}
}

// MatchRule maintains backward compatibility - returns true if any severity is triggered
func MatchRule(rule models.AlertRule, host models.Host, value float64) bool {
	return DetermineSeverity(rule, host, value) != SeverityNone
}

// ShouldActivateAlert determines if an alert should be activated (incident created).
func ShouldActivateAlert(rule models.AlertRule, host models.Host, value float64) bool {
	return DetermineSeverity(rule, host, value) != SeverityNone
}

// ShouldResolveAlertSeverity determines if an open alert with given severity should be resolved.
// Uses hysteresis thresholds if available, otherwise uses lower severity as clearance condition.
func ShouldResolveAlertSeverity(rule models.AlertRule, host models.Host, value float64, currentSeverity AlertSeverity) bool {
	if rule.Metric == "status_offline" {
		return host.Status != "offline"
	}

	// Determine what severity level would be active at the current value
	activeSeverity := DetermineSeverity(rule, host, value)

	if currentSeverity == SeverityCrit {
		// For critical incidents:
		// If threshold_clear_crit is set, resolve when value crosses it
		if rule.ThresholdClearCrit != nil {
			return resolvesHysteresis(rule.Operator, value, *rule.ThresholdClearCrit)
		}
		// Otherwise resolve if we drop to warn or below
		return activeSeverity != SeverityCrit
	}

	if currentSeverity == SeverityWarn {
		// For warning incidents:
		// If threshold_clear_warn is set, resolve when value crosses it
		if rule.ThresholdClearWarn != nil {
			return resolvesHysteresis(rule.Operator, value, *rule.ThresholdClearWarn)
		}
		// Otherwise resolve if no severity is active
		return activeSeverity == SeverityNone
	}

	return false
}

// resolvesHysteresis checks if value has crossed the clear threshold based on operator
func resolvesHysteresis(operator string, value float64, clearThreshold float64) bool {
	switch operator {
	case ">", ">=":
		return value <= clearThreshold
	case "<", "<=":
		return value >= clearThreshold
	default:
		return false
	}
}

func buildAlertMessage(rule models.AlertRule, host models.Host, value float64) string {
	if rule.Metric == "heartbeat_timeout" {
		totalSecs := int(value)
		if totalSecs >= 60 {
			return fmt.Sprintf("Agent silencieux sur %s depuis %dm%ds (dernier contact : %s)",
				host.Name, totalSecs/60, totalSecs%60, host.LastSeen.Local().Format("15:04:05"))
		}
		return fmt.Sprintf("Agent silencieux sur %s depuis %ds (dernier contact : %s)",
			host.Name, totalSecs, host.LastSeen.Local().Format("15:04:05"))
	}

	// Format Proxmox metrics in French with scope information
	if isProxmoxMetric(rule.Metric) {
		metricLabel := rule.Metric
		switch rule.Metric {
		case "proxmox_storage_percent":
			metricLabel = "Stockage Proxmox"
		case "proxmox_node_cpu_percent":
			metricLabel = "CPU noeud Proxmox"
		case "proxmox_node_memory_percent":
			metricLabel = "RAM noeud Proxmox"
		case "proxmox_node_cpu_temperature":
			metricLabel = "Temp. CPU noeud Proxmox"
		case "proxmox_node_fan_rpm":
			metricLabel = "RPM ventilateurs noeud Proxmox"
		case "proxmox_guest_cpu_percent":
			metricLabel = "CPU VM/LXC Proxmox"
		case "proxmox_guest_memory_percent":
			metricLabel = "RAM VM/LXC Proxmox"
		case "proxmox_node_pending_updates":
			metricLabel = "Paquets APT en attente"
		case "proxmox_recent_failed_tasks_24h":
			metricLabel = "Tâches Proxmox échouées (24h)"
		case "proxmox_auth_failures_recent":
			metricLabel = "Echecs auth Proxmox (logs)"
		case "proxmox_disk_failed_count":
			metricLabel = "Disques physiques en échec"
		case "proxmox_disk_min_wearout_percent":
			metricLabel = "Usure disque min"
		}
		switch rule.Metric {
		case "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_auth_failures_recent", "proxmox_disk_failed_count":
			return fmt.Sprintf("Alerte %s %s %.0f sur %s", metricLabel, rule.Operator, value, host.Name)
		case "proxmox_node_cpu_temperature":
			return fmt.Sprintf("Alerte %s %s %.1f°C sur %s", metricLabel, rule.Operator, value, host.Name)
		case "proxmox_node_fan_rpm":
			return fmt.Sprintf("Alerte %s %s %.0f RPM sur %s", metricLabel, rule.Operator, value, host.Name)
		default:
			return fmt.Sprintf("Alerte %s %s %.1f%% sur %s", metricLabel, rule.Operator, value, host.Name)
		}
	}

	return fmt.Sprintf("Alert %s %s %.2f on host %s (%s)", rule.Metric, rule.Operator, value, host.Name, host.ID)
}

func sendAlertNotifications(n notify.Notifier, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	msg := buildAlertMessage(rule, host, value)
	payload := map[string]interface{}{
		"title":          "ServerSupervisor Alert",
		"message":        msg,
		"rule_id":        rule.ID,
		"host_id":        host.ID,
		"host_name":      host.Name,
		"metric":         rule.Metric,
		"operator":       rule.Operator,
		"threshold_warn": rule.ThresholdWarn,
		"threshold_crit": rule.ThresholdCrit,
		"value":          value,
		"triggered_at":   time.Now().UTC(),
	}

	for _, channel := range rule.Actions.Channels {
		switch channel {
		case "smtp":
			to := rule.Actions.SMTPTo
			if to == "" {
				to = cfg.SMTPTo
			}
			if to == "" || cfg.SMTPFrom == "" {
				log.Printf("Alerts: SMTP to/from not configured for rule %d", rule.ID)
				continue
			}
			if err := n.SendSMTP(cfg, cfg.SMTPFrom, to, "[ServerSupervisor] Alert triggered", msg); err != nil {
				log.Printf("Alerts: SMTP send failed for rule %d: %v", rule.ID, err)
			}

		case "ntfy":
			topic := rule.Actions.NtfyTopic
			if topic == "" {
				log.Printf("Alerts: ntfy topic not configured for rule %d", rule.ID)
				continue
			}
			ntfyURL := "https://ntfy.sh/" + topic
			if err := n.SendNtfy(cfg, ntfyURL, "ServerSupervisor Alert", msg); err != nil {
				log.Printf("Alerts: ntfy failed for rule %d: %v", rule.ID, err)
			}

		case "notify":
			// Legacy webhook channel — send to configured notify URL
			notifyURL := cfg.NotifyURL
			if notifyURL == "" {
				log.Printf("Alerts: notify URL not configured")
				continue
			}
			data, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", notifyURL, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 10 * time.Second}
			if resp, err := client.Do(req); err != nil {
				log.Printf("Alerts: notify failed: %v", err)
			} else {
				_ = resp.Body.Close()
			}

		case "browser":
			// Browser notifications are delivered via the WebSocket push mechanism; no backend action needed here.

		default:
			log.Printf("Alerts: unknown channel %q for rule %d", channel, rule.ID)
		}
	}
}

// pushWebNotifications delivers a Web Push notification to every registered device subscription
// when a rule with the "browser" channel fires. Runs in a goroutine; VAPID keys are fetched
// from the settings table (generated on first alert if not yet present).
func pushWebNotifications(ctx context.Context, db *database.DB, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	hasBrowser := false
	for _, ch := range rule.Actions.Channels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	privateKey, err := db.GetSetting(ctx, "vapid_private_key")
	if err != nil || privateKey == "" {
		return
	}
	publicKey, err := db.GetSetting(ctx, "vapid_public_key")
	if err != nil || publicKey == "" {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.ThresholdCrit != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (crit)", rule.Metric, rule.Operator, *rule.ThresholdCrit)
	} else if rule.ThresholdWarn != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (warn)", rule.Metric, rule.Operator, *rule.ThresholdWarn)
	}

	unit := ""
	switch rule.Metric {
	case "cpu", "memory", "disk":
		unit = "%"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"title": "Alerte : " + ruleName,
		"body":  fmt.Sprintf("%s — Valeur : %.2f%s", host.Name, value, unit),
		"tag":   fmt.Sprintf("alert-%d-%s", rule.ID, host.ID),
		"url":   "/alerts?tab=incidents",
	})

	subs, err := db.GetPushSubscriptionsByRole(ctx, "admin")
	if err != nil || len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DHKey,
				Auth:   sub.AuthKey,
			},
		}
		resp, sendErr := webpush.SendNotification(payload, wpSub, &webpush.Options{
			Subscriber:      cfg.BaseURL,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             120,
		})
		if sendErr != nil {
			log.Printf("Push: delivery failed (%s…): %v", truncateStr(sub.Endpoint, 40), sendErr)
			if resp != nil && resp.StatusCode == http.StatusGone {
				_ = db.DeletePushSubscription(ctx, sub.Endpoint)
			}
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// pushBrowserNotification sends a real-time WebSocket event to all connected frontend clients
// when a rule with the "browser" channel fires. Safe to call with a nil pusher.
func pushBrowserNotification(pusher NotificationPusher, rule models.AlertRule, host models.Host, value float64, incID int64) {
	if pusher == nil {
		return
	}
	hasBrowser := false
	for _, ch := range rule.Actions.Channels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.ThresholdCrit != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (crit)", rule.Metric, rule.Operator, *rule.ThresholdCrit)
	} else if rule.ThresholdWarn != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (warn)", rule.Metric, rule.Operator, *rule.ThresholdWarn)
	}

	pusher.Broadcast(map[string]interface{}{
		"type": "new_alert",
		"notification": map[string]interface{}{
			"id":             fmt.Sprintf("alert:%d", incID),
			"type":           "alert_incident",
			"rule_id":        rule.ID,
			"host_id":        host.ID,
			"host_name":      host.Name,
			"rule_name":      ruleName,
			"metric":         rule.Metric,
			"value":          value,
			"triggered_at":   time.Now().UTC(),
			"resolved_at":    nil,
			"browser_notify": true,
		},
	})
}

// broadcastIncidentUpdate pushes a lightweight WS event so the frontend can refresh its incidents list
// without a polling interval. Fired for both new incidents and resolutions, regardless of channels.
func broadcastIncidentUpdate(pusher NotificationPusher, event string, rule models.AlertRule, hostID string) {
	if pusher == nil {
		return
	}
	pusher.Broadcast(map[string]interface{}{
		"type":    "alert_incident_update",
		"event":   event, // "fired" | "resolved"
		"rule_id": rule.ID,
		"host_id": hostID,
	})
}

// triggerAlertCommand creates a remote command on the host when an alert fires,
// if the rule's actions include a CommandTrigger.
//
// For Proxmox-scoped alerts the engine builds a synthetic host (ID prefixed by
// "proxmox:") that does not exist in the hosts table. We resolve the target to
// the agent host linked to the Proxmox guest (when scope_mode=guest and a
// confirmed link exists); other Proxmox scopes have no unique linked host, so
// the trigger is skipped.
func triggerAlertCommand(ctx context.Context, dispatcher *dispatch.Dispatcher, db *database.DB, rule models.AlertRule, host models.Host) {
	if dispatcher == nil {
		return
	}
	ct := rule.Actions.CommandTrigger
	if ct == nil || ct.Module == "" || ct.Action == "" {
		return
	}

	targetHostID := host.ID
	targetLabel := host.Name
	if strings.HasPrefix(host.ID, "proxmox:") {
		parts := strings.SplitN(host.ID, ":", 3)
		if len(parts) != 3 || parts[1] != "guest" || parts[2] == "" {
			log.Printf("Alerts: rule#%d command_trigger skipped — no linked host for Proxmox scope %s", rule.ID, host.ID)
			return
		}
		link, err := db.GetProxmoxGuestLinkByGuest(ctx, parts[2])
		if err != nil || link == nil {
			log.Printf("Alerts: rule#%d command_trigger skipped — no host link for Proxmox guest %s", rule.ID, parts[2])
			return
		}
		if link.Status != "confirmed" {
			log.Printf("Alerts: rule#%d command_trigger skipped — link for Proxmox guest %s is not confirmed (status=%s)", rule.ID, parts[2], link.Status)
			return
		}
		targetHostID = link.HostID
		targetLabel = link.HostName
	}

	payload := ct.Payload
	if payload == "" {
		payload = "{}"
	}
	triggeredBy := fmt.Sprintf("alert:%d", rule.ID)
	if _, err := dispatcher.Create(ctx, dispatch.Request{
		HostID:      targetHostID,
		Module:      ct.Module,
		Action:      ct.Action,
		Target:      ct.Target,
		Payload:     payload,
		TriggeredBy: triggeredBy,
	}); err != nil {
		log.Printf("Alerts: failed to create command trigger for rule %d on host %s: %v", rule.ID, targetHostID, err)
	} else {
		log.Printf("Alerts: triggered command %s/%s on host %s (rule %d)", ct.Module, ct.Action, targetLabel, rule.ID)
	}
}
