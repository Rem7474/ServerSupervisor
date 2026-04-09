package alerts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
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

func EvaluateAlerts(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, pusher NotificationPusher) {
	rules, err := db.GetAlertRules()
	if err != nil {
		log.Printf("Alerts: failed to fetch rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	hosts, err := db.GetAllHosts()
	if err != nil {
		log.Printf("Alerts: failed to fetch hosts: %v", err)
		return
	}

	n := notify.New()

	for _, rule := range rules {
		if !rule.Enabled {
			resolvedCount, err := db.ResolveOpenAlertIncidentsByRule(rule.ID)
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

		// Build evaluation targets: real hosts for agent metrics, synthetic host for Proxmox metrics
		hostsForRule := buildAlertEvaluationTargets(rule, hosts)

		for _, host := range hostsForRule {
			if hasHostID(rule) && !isProxmoxMetric(rule.Metric) && *rule.HostID != host.ID {
				continue
			}

			value, ok := GetMetricValue(db, host, rule)
			if !ok {
				continue
			}

			matched := MatchRule(rule, host, value)
			inc, err := db.GetOpenAlertIncident(rule.ID, host.ID)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Alerts: failed to check incidents: %v", err)
				continue
			}

			if matched {
				if err == sql.ErrNoRows || inc == nil {
					incID, err := db.CreateAlertIncident(rule.ID, host.ID, value)
					if err != nil {
						log.Printf("Alerts: failed to create incident: %v", err)
						continue
					}
					log.Printf("Alerts: FIRED %s host=%s value=%.2f → incident#%d created", ruleName, host.Name, value, incID)
					details := fmt.Sprintf(`{"rule_id":%d,"metric":"%s","operator":"%s","value":%.4f}`, rule.ID, rule.Metric, rule.Operator, value)
					_, _ = db.CreateAuditLog("alert-engine", "alert_fired", host.ID, "", details, "success")
					sendAlertNotifications(n, cfg, rule, host, value)
					triggerAlertCommand(dispatcher, rule, host)
					pushBrowserNotification(pusher, rule, host, value, incID)
					go pushWebNotifications(db, cfg, rule, host, value)
					broadcastIncidentUpdate(pusher, "fired", rule, host.ID)
				}
			} else if inc != nil {
				_ = db.ResolveAlertIncident(inc.ID)
				log.Printf("Alerts: %s host=%s — resolved incident#%d", ruleName, host.Name, inc.ID)
				details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d}`, rule.ID, inc.ID)
				_, _ = db.CreateAuditLog("alert-engine", "alert_resolved", host.ID, "", details, "success")
				broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
			}
		}
	}
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
		"proxmox_disk_failed_count",
		"proxmox_disk_min_wearout_percent":
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
func buildAlertEvaluationTargets(rule models.AlertRule, hosts []models.Host) []models.Host {
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

// GetMetricValue retrieves the current value of a metric for a host according to a rule.
func GetMetricValue(db *database.DB, host models.Host, rule models.AlertRule) (float64, bool) {
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
			if link, err := db.GetProxmoxGuestLinkByHost(host.ID); err == nil && link != nil && link.Status == "confirmed" && link.MetricsSource == "auto" {
				cpuPct, memPct, ts, err := db.GetLatestProxmoxGuestMetricPercent(link.GuestID)
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

		metrics, err := db.GetLatestMetrics(host.ID)
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
			temp, ok := db.GetEffectiveHostCPUTemperature(host.ID, metrics.CPUTemperature)
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
		healthData, err := db.GetLatestDiskHealth(host.ID)
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
		healthData, err := db.GetLatestDiskHealth(host.ID)
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
		pct := resolveProxmoxStoragePercent(db, rule)
		return pct, true
	case "proxmox_node_cpu_percent":
		pct := resolveProxmoxNodeCPUPercent(db, rule)
		return pct, true
	case "proxmox_node_memory_percent":
		pct := resolveProxmoxNodeMemoryPercent(db, rule)
		return pct, true
	case "proxmox_node_cpu_temperature":
		return resolveProxmoxNodeCPUTemperature(db, rule), true
	case "proxmox_node_fan_rpm":
		return resolveProxmoxNodeFanRPM(db, rule), true
	case "proxmox_guest_cpu_percent":
		return resolveProxmoxGuestCPUPercent(db, rule), true
	case "proxmox_guest_memory_percent":
		return resolveProxmoxGuestMemoryPercent(db, rule), true
	case "proxmox_node_pending_updates":
		return resolveProxmoxNodePendingUpdates(db, rule), true
	case "proxmox_recent_failed_tasks_24h":
		return resolveProxmoxRecentFailedTasks24h(db, rule), true
	case "proxmox_disk_failed_count":
		return resolveProxmoxDiskFailedCount(db, rule), true
	case "proxmox_disk_min_wearout_percent":
		return resolveProxmoxDiskMinWearoutPercent(db, rule), true
	}
	return 0, false
}

func resolveProxmoxStoragePercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxStorageUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetMaxProxmoxStorageUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetMaxProxmoxStorageUsagePercentByNode(scope.NodeID)
	case "storage":
		if scope.StorageID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetProxmoxStorageUsagePercentByStorage(scope.StorageID)
	default:
		return db.GetMaxProxmoxStorageUsagePercent()
	}
}

// resolveProxmoxNodeCPUPercent returns the CPU usage for a Proxmox node metric
// based on the scope defined in the rule (global, connection, or node).
func resolveProxmoxNodeCPUPercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeCPUUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeCPUUsagePercent()
		}
		return db.GetMaxProxmoxNodeCPUUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeCPUUsagePercent()
		}
		return db.GetProxmoxNodeCPUUsagePercentByNode(scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeCPUUsagePercent()
	}
}

// resolveProxmoxNodeMemoryPercent returns the memory usage for a Proxmox node metric
// based on the scope defined in the rule (global, connection, or node).
func resolveProxmoxNodeMemoryPercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeMemoryUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeMemoryUsagePercent()
		}
		return db.GetMaxProxmoxNodeMemoryUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeMemoryUsagePercent()
		}
		return db.GetProxmoxNodeMemoryUsagePercentByNode(scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeMemoryUsagePercent()
	}
}

func resolveProxmoxNodeCPUTemperature(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeCPUTemperature()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeCPUTemperature()
		}
		return db.GetMaxProxmoxNodeCPUTemperatureByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeCPUTemperature()
		}
		return db.GetProxmoxNodeCPUTemperatureByNode(scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeCPUTemperature()
	}
}

func resolveProxmoxNodeFanRPM(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxNodeFanRPM()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxNodeFanRPM()
		}
		return db.GetMaxProxmoxNodeFanRPMByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxNodeFanRPM()
		}
		return db.GetProxmoxNodeFanRPMByNode(scope.NodeID)
	default:
		return db.GetMaxProxmoxNodeFanRPM()
	}
}

func resolveProxmoxGuestCPUPercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxGuestCPUUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent()
		}
		return db.GetMaxProxmoxGuestCPUUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent()
		}
		return db.GetMaxProxmoxGuestCPUUsagePercentByNode(scope.NodeID)
	case "guest":
		if scope.GuestID == "" {
			return db.GetMaxProxmoxGuestCPUUsagePercent()
		}
		cpu, _, _, err := db.GetLatestProxmoxGuestMetricPercent(scope.GuestID)
		if err != nil {
			return 0
		}
		return cpu
	default:
		return db.GetMaxProxmoxGuestCPUUsagePercent()
	}
}

func resolveProxmoxGuestMemoryPercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxGuestMemoryUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent()
		}
		return db.GetMaxProxmoxGuestMemoryUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent()
		}
		return db.GetMaxProxmoxGuestMemoryUsagePercentByNode(scope.NodeID)
	case "guest":
		if scope.GuestID == "" {
			return db.GetMaxProxmoxGuestMemoryUsagePercent()
		}
		_, mem, _, err := db.GetLatestProxmoxGuestMetricPercent(scope.GuestID)
		if err != nil {
			return 0
		}
		return mem
	default:
		return db.GetMaxProxmoxGuestMemoryUsagePercent()
	}
}

func resolveProxmoxNodePendingUpdates(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(db.GetMaxProxmoxNodePendingUpdates())
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(db.GetMaxProxmoxNodePendingUpdates())
		}
		return float64(db.GetMaxProxmoxNodePendingUpdatesByConnection(scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(db.GetMaxProxmoxNodePendingUpdates())
		}
		return float64(db.GetMaxProxmoxNodePendingUpdatesByNode(scope.NodeID))
	default:
		return float64(db.GetMaxProxmoxNodePendingUpdates())
	}
}

func resolveProxmoxRecentFailedTasks24h(db *database.DB, rule models.AlertRule) float64 {
	since := time.Now().Add(-24 * time.Hour)
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		count, err := db.GetRecentFailedTaskCount(since)
		if err != nil {
			return 0
		}
		return float64(count)
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			count, err := db.GetRecentFailedTaskCount(since)
			if err != nil {
				return 0
			}
			return float64(count)
		}
		count, err := db.GetRecentFailedTaskCountByConnection(scope.ConnectionID, since)
		if err != nil {
			return 0
		}
		return float64(count)
	case "node":
		if scope.NodeID == "" {
			count, err := db.GetRecentFailedTaskCount(since)
			if err != nil {
				return 0
			}
			return float64(count)
		}
		count, err := db.GetRecentFailedTaskCountByNodeID(scope.NodeID, since)
		if err != nil {
			return 0
		}
		return float64(count)
	default:
		count, err := db.GetRecentFailedTaskCount(since)
		if err != nil {
			return 0
		}
		return float64(count)
	}
}

func resolveProxmoxDiskFailedCount(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(db.GetProxmoxDiskFailedCount())
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(db.GetProxmoxDiskFailedCount())
		}
		return float64(db.GetProxmoxDiskFailedCountByConnection(scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(db.GetProxmoxDiskFailedCount())
		}
		return float64(db.GetProxmoxDiskFailedCountByNodeID(scope.NodeID))
	case "disk":
		if scope.DiskID == "" {
			return float64(db.GetProxmoxDiskFailedCount())
		}
		return float64(db.GetProxmoxDiskFailedCountByDiskID(scope.DiskID))
	default:
		return float64(db.GetProxmoxDiskFailedCount())
	}
}

func resolveProxmoxDiskMinWearoutPercent(db *database.DB, rule models.AlertRule) float64 {
	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetProxmoxDiskMinWearoutPercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetProxmoxDiskMinWearoutPercent()
		}
		return db.GetProxmoxDiskMinWearoutPercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetProxmoxDiskMinWearoutPercent()
		}
		return db.GetProxmoxDiskMinWearoutPercentByNodeID(scope.NodeID)
	case "disk":
		if scope.DiskID == "" {
			return db.GetProxmoxDiskMinWearoutPercent()
		}
		return db.GetProxmoxDiskWearoutPercentByDiskID(scope.DiskID)
	default:
		return db.GetProxmoxDiskMinWearoutPercent()
	}
}

// MatchRule evaluates whether a rule condition is currently met for the given value.
func MatchRule(rule models.AlertRule, host models.Host, value float64) bool {
	if rule.Metric == "status_offline" {
		return host.Status == "offline"
	}
	if rule.Threshold == nil {
		return false
	}

	switch rule.Operator {
	case ">":
		return value > *rule.Threshold
	case "<":
		return value < *rule.Threshold
	case ">=":
		return value >= *rule.Threshold
	case "<=":
		return value <= *rule.Threshold
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
		case "proxmox_disk_failed_count":
			metricLabel = "Disques physiques en échec"
		case "proxmox_disk_min_wearout_percent":
			metricLabel = "Usure disque min"
		}
		switch rule.Metric {
		case "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_disk_failed_count":
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
		"title":        "ServerSupervisor Alert",
		"message":      msg,
		"rule_id":      rule.ID,
		"host_id":      host.ID,
		"host_name":    host.Name,
		"metric":       rule.Metric,
		"operator":     rule.Operator,
		"threshold":    rule.Threshold,
		"value":        value,
		"triggered_at": time.Now().UTC(),
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
func pushWebNotifications(db *database.DB, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
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

	privateKey, err := db.GetSetting("vapid_private_key")
	if err != nil || privateKey == "" {
		return
	}
	publicKey, err := db.GetSetting("vapid_public_key")
	if err != nil || publicKey == "" {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.Threshold != nil {
		ruleName = fmt.Sprintf("%s %s %.2f", rule.Metric, rule.Operator, *rule.Threshold)
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

	subs, err := db.GetPushSubscriptionsByRole("admin")
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
				_ = db.DeletePushSubscription(sub.Endpoint)
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
	} else if rule.Threshold != nil {
		ruleName = fmt.Sprintf("%s %s %.2f", rule.Metric, rule.Operator, *rule.Threshold)
	}

	pusher.Broadcast(map[string]interface{}{
		"type": "new_alert",
		"notification": map[string]interface{}{
			"id":             incID,
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
func triggerAlertCommand(dispatcher *dispatch.Dispatcher, rule models.AlertRule, host models.Host) {
	if dispatcher == nil {
		return
	}
	ct := rule.Actions.CommandTrigger
	if ct == nil || ct.Module == "" || ct.Action == "" {
		return
	}
	payload := ct.Payload
	if payload == "" {
		payload = "{}"
	}
	triggeredBy := fmt.Sprintf("alert:%d", rule.ID)
	if _, err := dispatcher.Create(dispatch.Request{
		HostID:      host.ID,
		Module:      ct.Module,
		Action:      ct.Action,
		Target:      ct.Target,
		Payload:     payload,
		TriggeredBy: triggeredBy,
	}); err != nil {
		log.Printf("Alerts: failed to create command trigger for rule %d on host %s: %v", rule.ID, host.ID, err)
	} else {
		log.Printf("Alerts: triggered command %s/%s on host %s (rule %d)", ct.Module, ct.Action, host.Name, rule.ID)
	}
}
