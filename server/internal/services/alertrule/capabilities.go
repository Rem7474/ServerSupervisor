package alertrule

import (
	"context"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// ===== static metric catalogs =====

// AgentCapabilities returns the agent (per-host) metric catalog.
func (s *Service) AgentCapabilities() []models.AlertMetricCapability {
	return []models.AlertMetricCapability{
		{Metric: "cpu", Label: "CPU", Unit: "%", Icon: "⚡", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "cpu_temperature", Label: "Temp. CPU", Unit: "°C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "memory", Label: "RAM", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "disk", Label: "Disque", Unit: "%", Icon: "\U0001f4be", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "load", Label: "Load avg", Unit: "", Icon: "\U0001f4c8", BadgeClass: "bg-purple-lt text-purple", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "heartbeat_timeout", Label: "Heartbeat", Unit: "s", Icon: "\U0001fac0", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "status_offline", Label: "Hote hors ligne", Unit: "", Icon: "\U0001f50c", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_smart_status", Label: "SMART disque", Unit: "", Icon: "\U0001f6e1", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_temperature", Label: "Temp. disque", Unit: "°C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
	}
}

// ProxmoxMetrics returns the Proxmox metric catalog.
func proxmoxMetrics() []models.AlertMetricCapability {
	return []models.AlertMetricCapability{
		{Metric: "proxmox_storage_percent", Label: "Proxmox stockage", Unit: "%", Icon: "\U0001f5a5", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_cpu_percent", Label: "Proxmox CPU noeud", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_memory_percent", Label: "Proxmox RAM noeud", Unit: "%", Icon: "\U0001f4ca", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_cpu_temperature", Label: "Proxmox temp. CPU noeud", Unit: "°C", Icon: "\U0001f321", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_fan_rpm", Label: "Proxmox RPM ventilateurs noeud", Unit: " RPM", Icon: "\U0001f300", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_guest_cpu_percent", Label: "CPU VM/LXC Proxmox", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_guest_memory_percent", Label: "RAM VM/LXC Proxmox", Unit: "%", Icon: "\U0001f4ca", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_pending_updates", Label: "Paquets APT en attente", Unit: "", Icon: "\U0001f504", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_recent_failed_tasks_24h", Label: "Tâches Proxmox échouées (24h)", Unit: "", Icon: "\U0001f552", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_auth_failures_recent", Label: "Echecs auth Proxmox (logs)", Unit: "", Icon: "\U0001f512", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_disk_failed_count", Label: "Disques physiques en échec", Unit: "", Icon: "\U0001f4a5", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_disk_min_wearout_percent", Label: "Usure disque min", Unit: "%", Icon: "\U0001f6e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
	}
}

// SyntheticCapabilities returns the synthetic-monitoring metric catalog.
func (s *Service) SyntheticCapabilities() []models.AlertMetricCapability {
	return []models.AlertMetricCapability{
		{Metric: "uptime_down_count", Label: "Sondes uptime down", Unit: "", Icon: "\U0001f6a8", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "ssl_min_days_remaining", Label: "Cert SSL — jours restants", Unit: "j", Icon: "\U0001f510", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: false},
	}
}

func dockerMetrics() []models.AlertMetricCapability {
	return []models.AlertMetricCapability{
		{Metric: "docker_container_state", Label: "État d'un container", Unit: "", Icon: "🐳", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "docker_compose_degraded_services", Label: "Services Compose dégradés", Unit: "", Icon: "🐳", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
	}
}

// ===== capability responses =====

// ProxmoxCapabilities returns the Proxmox metric catalog + the scope options.
func (s *Service) ProxmoxCapabilities(ctx context.Context) (models.AlertSplitCapabilities, error) {
	resp := models.AlertSplitCapabilities{
		AgentMetrics:   []models.AlertMetricCapability{},
		ProxmoxMetrics: proxmoxMetrics(),
	}
	resp.ProxmoxScope.Modes = []string{"global", "connection", "node", "storage", "guest", "disk"}
	resp.ProxmoxScope.Connections, _ = s.repo.ListAlertProxmoxConnections(ctx)
	resp.ProxmoxScope.Nodes, _ = s.repo.ListAlertProxmoxNodes(ctx)
	resp.ProxmoxScope.Storages, _ = s.repo.ListAlertProxmoxStorages(ctx)
	resp.ProxmoxScope.Guests, _ = s.repo.ListAlertProxmoxGuests(ctx)
	resp.ProxmoxScope.Disks, _ = s.repo.ListAlertProxmoxDisks(ctx)
	return resp, nil
}

// DockerCapabilities returns the Docker metric catalog + per-host scope options.
func (s *Service) DockerCapabilities(ctx context.Context) (models.AlertDockerCapabilities, error) {
	hosts := []models.AlertDockerHostScope{}
	hostOpts, _ := s.repo.ListAlertDockerScopeHosts(ctx)
	for _, ho := range hostOpts {
		scope := models.AlertDockerHostScope{
			HostID:     ho.ID,
			HostName:   ho.Label,
			Containers: []models.AlertDockerScopeContainer{},
			Projects:   []models.AlertDockerScopeProject{},
		}
		containers, _ := s.repo.GetDockerContainers(ctx, ho.ID)
		for _, ct := range containers {
			label := ct.Image
			if ct.ImageTag != "" && ct.ImageTag != "latest" {
				label = ct.Image + ":" + ct.ImageTag
			}
			scope.Containers = append(scope.Containers, models.AlertDockerScopeContainer{
				ID: ct.ID, Name: ct.Name, Image: label, State: ct.State,
			})
		}
		projects, _ := s.repo.GetComposeProjectsByHost(ctx, ho.ID)
		for _, p := range projects {
			services := p.Services
			if services == nil {
				services = []string{}
			}
			scope.Projects = append(scope.Projects, models.AlertDockerScopeProject{Name: p.Name, Services: services})
		}
		hosts = append(hosts, scope)
	}
	return models.AlertDockerCapabilities{Metrics: dockerMetrics(), Hosts: hosts}, nil
}

// HostMetrics returns the agent metrics available on a host given its collectors.
func (s *Service) HostMetrics(ctx context.Context, hostID string) (models.AlertHostCapabilities, error) {
	host, err := s.repo.GetHost(ctx, hostID)
	if err != nil {
		return models.AlertHostCapabilities{}, apperr.NotFound("host not found")
	}
	return models.AlertHostCapabilities{
		HostID:   host.ID,
		HostName: host.Name,
		Metrics:  filterMetricsByCollectors(s.AgentCapabilities(), host.Collectors),
	}, nil
}

// filterMetricsByCollectors keeps the base metrics plus those whose required
// collector is enabled on the host.
func filterMetricsByCollectors(all []models.AlertMetricCapability, collectors map[string]bool) []models.AlertMetricCapability {
	alwaysAvailable := map[string]bool{
		"cpu": true, "memory": true, "disk": true, "load": true,
		"heartbeat_timeout": true, "status_offline": true,
	}
	requiresCollector := map[string]string{
		"cpu_temperature":   "cpu_temp",
		"disk_smart_status": "smart",
		"disk_temperature":  "smart",
	}
	var filtered []models.AlertMetricCapability
	for _, metric := range all {
		if alwaysAvailable[metric.Metric] {
			filtered = append(filtered, metric)
			continue
		}
		if required, ok := requiresCollector[metric.Metric]; ok && collectors[required] {
			filtered = append(filtered, metric)
		}
	}
	return filtered
}

// ===== scope test target (rule test preview labelling) =====

// ProxmoxScopeTestTarget resolves a Proxmox scope to a (targetID, label) pair for
// the rule test preview.
func (s *Service) ProxmoxScopeTestTarget(ctx context.Context, scope *models.ProxmoxMetricScope) (string, string) {
	const globalID, globalLabel = "proxmox:global", "Cluster Proxmox"
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return globalID, globalLabel
	}
	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return globalID, globalLabel
		}
		if name, err := s.repo.ProxmoxConnectionName(ctx, scope.ConnectionID); err == nil && strings.TrimSpace(name) != "" {
			return "proxmox:connection:" + scope.ConnectionID, "Connexion: " + name
		}
		return "proxmox:connection:" + scope.ConnectionID, "Connexion: " + scope.ConnectionID
	case "node":
		if scope.NodeID == "" {
			return globalID, globalLabel
		}
		if connName, nodeName, err := s.repo.ProxmoxNodeLabelParts(ctx, scope.NodeID); err == nil {
			if strings.TrimSpace(connName) != "" {
				return "proxmox:node:" + scope.NodeID, "Noeud: " + connName + " / " + nodeName
			}
			return "proxmox:node:" + scope.NodeID, "Noeud: " + nodeName
		}
		return "proxmox:node:" + scope.NodeID, "Noeud: " + scope.NodeID
	case "storage":
		if scope.StorageID == "" {
			return globalID, globalLabel
		}
		if connName, nodeName, storageName, err := s.repo.ProxmoxStorageLabelParts(ctx, scope.StorageID); err == nil {
			if strings.TrimSpace(connName) != "" {
				return "proxmox:storage:" + scope.StorageID, "Stockage: " + connName + " / " + nodeName + " / " + storageName
			}
			return "proxmox:storage:" + scope.StorageID, "Stockage: " + nodeName + " / " + storageName
		}
		return "proxmox:storage:" + scope.StorageID, "Stockage: " + scope.StorageID
	case "guest":
		if scope.GuestID == "" {
			return globalID, globalLabel
		}
		if connName, nodeName, guestName, guestType, vmid, err := s.repo.ProxmoxGuestLabelParts(ctx, scope.GuestID); err == nil {
			suffix := fmt.Sprintf("%s:%d", strings.ToUpper(guestType), vmid)
			if strings.TrimSpace(connName) != "" {
				return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + connName + " / " + nodeName + " / " + guestName + " (" + suffix + ")"
			}
			return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + nodeName + " / " + guestName + " (" + suffix + ")"
		}
		return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + scope.GuestID
	case "disk":
		if scope.DiskID == "" {
			return globalID, globalLabel
		}
		if connName, nodeName, devPath, model, err := s.repo.ProxmoxDiskLabelParts(ctx, scope.DiskID); err == nil {
			detail := devPath
			if strings.TrimSpace(model) != "" {
				detail = model + " (" + devPath + ")"
			}
			if strings.TrimSpace(connName) != "" {
				return "proxmox:disk:" + scope.DiskID, "Disque: " + connName + " / " + nodeName + " / " + detail
			}
			return "proxmox:disk:" + scope.DiskID, "Disque: " + nodeName + " / " + detail
		}
		return "proxmox:disk:" + scope.DiskID, "Disque: " + scope.DiskID
	default:
		return globalID, globalLabel
	}
}
