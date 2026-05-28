package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// reportProxmoxIsMetricsSource reports whether Proxmox is the exclusive metrics
// source for this host, in which case the server skips storing agent CPU/RAM and
// signals the agent to stop collecting them.
//
// metrics_source semantics:
//
//	"proxmox" → Proxmox is the sole source.
//	"auto"    → Proxmox preferred but only when its data is fresh (within 3× poll
//	            interval); otherwise the agent resumes as fallback.
//	"agent"   → agent is always used; Proxmox ignored.
//
// A host used as a Proxmox CPU-temperature or fan-RPM source must keep sending
// local metrics so those sensors resolve for all linked guests on that node.
func (h *AgentHandler) reportProxmoxIsMetricsSource(ctx context.Context, hostID string) bool {
	proxmoxIsMetricsSource := false
	if link, err := h.db.GetProxmoxGuestLinkByHost(ctx, hostID); err == nil && link != nil {
		switch link.MetricsSource {
		case "proxmox":
			proxmoxIsMetricsSource = true
		case "auto":
			if fresh, err := h.db.IsProxmoxGuestDataFresh(ctx, hostID); err == nil {
				proxmoxIsMetricsSource = fresh
			}
		}
	}
	if proxmoxIsMetricsSource && h.db.IsHostUsedAsProxmoxCPUTempSource(ctx, hostID) {
		proxmoxIsMetricsSource = false
	}
	if proxmoxIsMetricsSource && h.db.IsHostUsedAsProxmoxFanRPMSource(ctx, hostID) {
		proxmoxIsMetricsSource = false
	}
	return proxmoxIsMetricsSource
}

// storeAgentMetrics persists host info + metrics from the report. It returns a
// non-nil error only for the fatal cases that must surface as HTTP 500 (uptime
// or metrics insert failure); host-info update failures are logged and ignored.
func (h *AgentHandler) storeAgentMetrics(ctx context.Context, hostID string, report *models.AgentReport, proxmoxIsMetricsSource bool) error {
	if report.Metrics != nil {
		update := models.HostUpdate{
			Hostname:     stringPtrIfNotEmpty(report.Metrics.Hostname),
			OS:           stringPtrIfNotEmpty(report.Metrics.OS),
			AgentVersion: stringPtrIfNotEmpty(report.AgentVersion),
		}
		if update.Hostname != nil || update.OS != nil || update.AgentVersion != nil {
			if err := h.db.UpdateHost(ctx, hostID, &update); err != nil {
				log.Printf("Warning: failed to update host %s: %v", hostID, err)
			}
		}

		// Skip storing metrics when Proxmox is the designated source —
		// Proxmox polling already stores CPU/RAM for this host.
		if proxmoxIsMetricsSource {
			if err := h.db.InsertUptimeMetrics(ctx, hostID, report.Metrics.Uptime, report.Metrics.Hostname); err != nil {
				return fmt.Errorf("failed to store uptime")
			}
		} else {
			report.Metrics.HostID = hostID
			report.Metrics.Timestamp = time.Now()
			if _, err := h.db.InsertMetrics(ctx, report.Metrics); err != nil {
				return fmt.Errorf("failed to store metrics")
			}
		}
		return nil
	}

	// No metrics — still update agent version when provided.
	if report.AgentVersion != "" {
		update := models.HostUpdate{AgentVersion: stringPtrIfNotEmpty(report.AgentVersion)}
		if err := h.db.UpdateHost(ctx, hostID, &update); err != nil {
			log.Printf("Warning: failed to update host %s: %v", hostID, err)
		}
	}
	return nil
}

// storeContainersAndPackages persists Docker containers/networks/compose
// projects and APT/unattended-upgrades state. All failures are non-fatal (logged).
func (h *AgentHandler) storeContainersAndPackages(ctx context.Context, hostID, safeHostID string, report *models.AgentReport) {
	if report.Docker != nil {
		for i := range report.Docker.Containers {
			report.Docker.Containers[i].HostID = hostID
		}
		if err := h.db.UpsertDockerContainers(ctx, hostID, report.Docker.Containers); err != nil {
			log.Printf("Warning: failed to store docker containers for host %s: %v", safeHostID, err)
		}
	}

	if report.AptStatus != nil {
		report.AptStatus.HostID = hostID
		if err := h.db.UpsertAptStatus(ctx, report.AptStatus); err != nil {
			log.Printf("Warning: failed to store apt status for host %s: %v", safeHostID, err)
		}
	}

	if report.UnattendedUpgrades != nil {
		if err := h.db.UpsertUUStatus(ctx, hostID, *report.UnattendedUpgrades); err != nil {
			log.Printf("Warning: failed to store UU status for host %s: %v", safeHostID, err)
		}
		for _, run := range report.UnattendedUpgrades.NewRuns {
			isNew, err := h.db.InsertUURunIfNew(ctx, hostID, run)
			if err != nil {
				log.Printf("Warning: failed to insert UU run for host %s: %v", safeHostID, err)
				continue
			}
			if isNew {
				_ = h.db.UpdateUULastRun(ctx, hostID, run.RunAt, len(run.Packages))
				_ = h.db.TouchAptLastAction(ctx, hostID, "update")
				if len(run.Packages) > 0 {
					_ = h.db.TouchAptLastUpgradeAt(ctx, hostID, run.RunAt)
					hostname := hostID
					if host, err := h.db.GetHost(ctx, hostID); err == nil && host != nil {
						hostname = host.Hostname
					}
					h.pushUUNotification(hostname, hostID, run)
				}
			}
		}
	}

	if report.DockerNetworks != nil {
		dbNetworks := make([]models.DockerNetwork, 0, len(report.DockerNetworks))
		for _, n := range report.DockerNetworks {
			dbNetworks = append(dbNetworks, models.DockerNetwork{
				ID:           fmt.Sprintf("%s-%s", hostID, n.NetworkID),
				HostID:       hostID,
				NetworkID:    n.NetworkID,
				Name:         n.Name,
				Driver:       n.Driver,
				Scope:        n.Scope,
				ContainerIDs: n.ContainerIDs,
				UpdatedAt:    time.Now(),
			})
		}
		if err := h.db.UpsertDockerNetworks(ctx, hostID, dbNetworks); err != nil {
			log.Printf("Warning: failed to store docker networks for host %s: %v", safeHostID, err)
		}
	}

	if report.ComposeProjects != nil {
		if err := h.db.UpsertComposeProjects(ctx, hostID, report.ComposeProjects); err != nil {
			log.Printf("Warning: failed to store compose projects for host %s: %v", safeHostID, err)
		}
	}
}

// storeDiskAndMetadata persists disk metrics/health and host metadata (custom
// tasks, tasks.yaml, collector capabilities, web logs). All failures are
// non-fatal (logged).
func (h *AgentHandler) storeDiskAndMetadata(ctx context.Context, hostID, safeHostID string, report *models.AgentReport) {
	if len(report.DiskMetrics) > 0 {
		batchTime := time.Now()
		for i := range report.DiskMetrics {
			report.DiskMetrics[i].HostID = hostID
			report.DiskMetrics[i].Timestamp = batchTime
		}
		if err := h.db.InsertDiskMetrics(ctx, report.DiskMetrics); err != nil {
			log.Printf("Warning: failed to store disk metrics for host %s: %v", safeHostID, err)
		}
	}

	if len(report.DiskHealth) > 0 {
		for i := range report.DiskHealth {
			report.DiskHealth[i].HostID = hostID
			report.DiskHealth[i].CollectedAt = time.Now()
		}
		if err := h.db.InsertDiskHealth(ctx, report.DiskHealth); err != nil {
			log.Printf("Warning: failed to store disk health for host %s: %v", safeHostID, err)
		}
	}

	if report.CustomTasks != nil {
		if b, err := json.Marshal(report.CustomTasks); err == nil {
			if err := h.db.UpdateHostCustomTasks(ctx, hostID, string(b)); err != nil {
				log.Printf("Warning: failed to store custom tasks for host %s: %v", safeHostID, err)
			}
		}
	}

	if report.TasksConfigYAML != "" {
		if err := h.db.UpdateHostTasksConfigYAML(ctx, hostID, report.TasksConfigYAML); err != nil {
			log.Printf("Warning: failed to store tasks config YAML for host %s: %v", safeHostID, err)
		}
	}

	if report.Capabilities != nil {
		if b, err := json.Marshal(report.Capabilities); err == nil {
			if err := h.db.UpdateHostCollectors(ctx, hostID, string(b)); err != nil {
				log.Printf("Warning: failed to store collectors for host %s: %v", safeHostID, err)
			}
		}
	}

	if report.WebLogs != nil {
		if err := h.db.UpdateHostWebLogs(ctx, hostID, report.WebLogs); err != nil {
			log.Printf("Warning: failed to update web logs cache for host %s: %v", safeHostID, err)
		}
		if err := h.db.InsertWebLogSnapshot(ctx, hostID, report.WebLogs); err != nil {
			log.Printf("Warning: failed to insert web logs snapshot for host %s: %v", safeHostID, err)
		}
	}
}
