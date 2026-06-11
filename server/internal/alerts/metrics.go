package alerts

import (
	"context"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

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
			// Disk usage now lives in disk_metrics (the legacy disk_info table was
			// removed in V2); take the worst mount point for the host.
			disks, err := db.GetLatestDiskMetrics(ctx, host.ID)
			if err != nil || len(disks) == 0 {
				return 0, false
			}
			maxDisk := 0.0
			for _, d := range disks {
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
	case "docker_container_not_running":
		// host.ID is "docker:container:<db-uuid>"; value 1 = not running, 0 = running.
		containerID := strings.TrimPrefix(host.ID, "docker:container:")
		c, err := db.GetDockerContainerByID(ctx, containerID)
		if err != nil || c == nil {
			return 0, false
		}
		if c.State != "running" {
			return 1, true
		}
		return 0, true
	case "docker_container_running_count":
		// host.ID is "docker:host:<host-id>"; value = count of running containers.
		hostID := strings.TrimPrefix(host.ID, "docker:host:")
		count, err := db.CountRunningDockerContainersByHost(ctx, hostID)
		if err != nil {
			return 0, false
		}
		return float64(count), true
	case "docker_compose_degraded_services":
		// host.ID is "docker:compose:<host-id>:<project-name>"; value = declared - running service count.
		rest := strings.TrimPrefix(host.ID, "docker:compose:")
		idx := strings.Index(rest, ":")
		if idx < 0 {
			return 0, false
		}
		hostID, projectName := rest[:idx], rest[idx+1:]
		declared, running, err := db.GetDockerComposeServiceCounts(ctx, hostID, projectName)
		if err != nil {
			return 0, false
		}
		degraded := declared - running
		if degraded < 0 {
			degraded = 0
		}
		return float64(degraded), true
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
