package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
)

func (h *AlertRulesHandler) proxmoxScopeTestTarget(ctx context.Context, scope *models.ProxmoxMetricScope) (string, string) {
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return "proxmox:global", "Cluster Proxmox"
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return "proxmox:global", "Cluster Proxmox"
		}
		var connName string
		if err := h.db.QueryRow(ctx, `SELECT name FROM proxmox_connections WHERE id = $1`, scope.ConnectionID).Scan(&connName); err == nil && strings.TrimSpace(connName) != "" {
			return "proxmox:connection:" + scope.ConnectionID, "Connexion: " + connName
		}
		return "proxmox:connection:" + scope.ConnectionID, "Connexion: " + scope.ConnectionID
	case "node":
		if scope.NodeID == "" {
			return "proxmox:global", "Cluster Proxmox"
		}
		var connName, nodeName string
		if err := h.db.QueryRow(ctx, `
			SELECT COALESCE(c.name, ''), n.node_name
			FROM proxmox_nodes n
			LEFT JOIN proxmox_connections c ON c.id = n.connection_id
			WHERE n.id = $1`, scope.NodeID).Scan(&connName, &nodeName); err == nil {
			if strings.TrimSpace(connName) != "" {
				return "proxmox:node:" + scope.NodeID, "Noeud: " + connName + " / " + nodeName
			}
			return "proxmox:node:" + scope.NodeID, "Noeud: " + nodeName
		}
		return "proxmox:node:" + scope.NodeID, "Noeud: " + scope.NodeID
	case "storage":
		if scope.StorageID == "" {
			return "proxmox:global", "Cluster Proxmox"
		}
		var connName, nodeName, storageName string
		if err := h.db.QueryRow(ctx, `
			SELECT COALESCE(c.name, ''), s.node_name, s.storage_name
			FROM proxmox_storages s
			LEFT JOIN proxmox_connections c ON c.id = s.connection_id
			WHERE s.id = $1`, scope.StorageID).Scan(&connName, &nodeName, &storageName); err == nil {
			if strings.TrimSpace(connName) != "" {
				return "proxmox:storage:" + scope.StorageID, "Stockage: " + connName + " / " + nodeName + " / " + storageName
			}
			return "proxmox:storage:" + scope.StorageID, "Stockage: " + nodeName + " / " + storageName
		}
		return "proxmox:storage:" + scope.StorageID, "Stockage: " + scope.StorageID
	case "guest":
		if scope.GuestID == "" {
			return "proxmox:global", "Cluster Proxmox"
		}
		var connName, nodeName, guestName, guestType string
		var vmid int
		if err := h.db.QueryRow(ctx, `
			SELECT COALESCE(c.name, ''), g.node_name, g.name, g.guest_type, g.vmid
			FROM proxmox_guests g
			LEFT JOIN proxmox_connections c ON c.id = g.connection_id
			WHERE g.id = $1`, scope.GuestID).Scan(&connName, &nodeName, &guestName, &guestType, &vmid); err == nil {
			suffix := fmt.Sprintf("%s:%d", strings.ToUpper(guestType), vmid)
			if strings.TrimSpace(connName) != "" {
				return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + connName + " / " + nodeName + " / " + guestName + " (" + suffix + ")"
			}
			return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + nodeName + " / " + guestName + " (" + suffix + ")"
		}
		return "proxmox:guest:" + scope.GuestID, "VM/LXC: " + scope.GuestID
	case "disk":
		if scope.DiskID == "" {
			return "proxmox:global", "Cluster Proxmox"
		}
		var connName, nodeName, devPath, model string
		if err := h.db.QueryRow(ctx, `
			SELECT COALESCE(c.name, ''), d.node_name, d.dev_path, d.model
			FROM proxmox_disks d
			LEFT JOIN proxmox_connections c ON c.id = d.connection_id
			WHERE d.id = $1`, scope.DiskID).Scan(&connName, &nodeName, &devPath, &model); err == nil {
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
		return "proxmox:global", "Cluster Proxmox"
	}
}

func (h *AlertRulesHandler) loadProxmoxScopeOptions(ctx context.Context) (modes []string, connections, nodes, storages, guests, disks []alertScopeOption) {
	modes = []string{"global", "connection", "node", "storage", "guest", "disk"}
	connections = []alertScopeOption{}
	nodes = []alertScopeOption{}
	storages = []alertScopeOption{}
	guests = []alertScopeOption{}
	disks = []alertScopeOption{}

	if rows, err := h.db.Query(ctx, `SELECT id, name FROM proxmox_connections ORDER BY name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, name string
			if scanErr := rows.Scan(&id, &name); scanErr == nil {
				connections = append(connections, alertScopeOption{ID: id, Label: name})
			}
		}
	}

	if rows, err := h.db.Query(ctx, `
		SELECT n.id, COALESCE(c.name,'?') || ' / ' || n.node_name
		FROM proxmox_nodes n
		LEFT JOIN proxmox_connections c ON c.id = n.connection_id
		ORDER BY c.name, n.node_name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				nodes = append(nodes, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	if rows, err := h.db.Query(ctx, `
		SELECT s.id, COALESCE(c.name,'?') || ' / ' || s.node_name || ' / ' || s.storage_name
		FROM proxmox_storages s
		LEFT JOIN proxmox_connections c ON c.id = s.connection_id
		ORDER BY c.name, s.node_name, s.storage_name`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				storages = append(storages, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	if rows, err := h.db.Query(ctx, `
		SELECT g.id,
		       COALESCE(c.name,'?') || ' / ' || g.node_name || ' / ' || COALESCE(NULLIF(g.name,''), '(sans nom)') || ' (' || UPPER(g.guest_type) || ':' || g.vmid || ')'
		FROM proxmox_guests g
		LEFT JOIN proxmox_connections c ON c.id = g.connection_id
		ORDER BY c.name, g.node_name, g.guest_type, g.vmid`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				guests = append(guests, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	if rows, err := h.db.Query(ctx, `
		SELECT d.id,
		       COALESCE(c.name,'?') || ' / ' || d.node_name || ' / ' ||
		       CASE
		         WHEN COALESCE(NULLIF(d.model,''),'') <> '' THEN d.model || ' (' || d.dev_path || ')'
		         ELSE d.dev_path
		       END
		FROM proxmox_disks d
		LEFT JOIN proxmox_connections c ON c.id = d.connection_id
		ORDER BY c.name, d.node_name, d.dev_path`); err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, label string
			if scanErr := rows.Scan(&id, &label); scanErr == nil {
				disks = append(disks, alertScopeOption{ID: id, Label: label})
			}
		}
	}

	return modes, connections, nodes, storages, guests, disks
}

type dockerScopeOptionContainer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	State string `json:"state"`
}

type dockerScopeOptionProject struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

type dockerHostScopeOption struct {
	HostID     string                       `json:"host_id"`
	HostName   string                       `json:"host_name"`
	Containers []dockerScopeOptionContainer `json:"containers"`
	Projects   []dockerScopeOptionProject   `json:"projects"`
}

type alertDockerCapabilitiesResponse struct {
	Metrics []alertMetricCapability `json:"metrics"`
	Hosts   []dockerHostScopeOption `json:"hosts"`
}

func (h *AlertRulesHandler) GetDockerAlertRuleCapabilities(c *gin.Context) {
	ctx := c.Request.Context()
	hosts := []dockerHostScopeOption{}

	rows, err := h.db.Query(ctx, `
		SELECT DISTINCT dc.host_id, h.hostname
		FROM docker_containers dc
		JOIN hosts h ON h.id = dc.host_id
		ORDER BY h.hostname`)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var hostID, hostName string
			if scanErr := rows.Scan(&hostID, &hostName); scanErr != nil {
				continue
			}
			opt := dockerHostScopeOption{
				HostID:     hostID,
				HostName:   hostName,
				Containers: []dockerScopeOptionContainer{},
				Projects:   []dockerScopeOptionProject{},
			}

			if cRows, cErr := h.db.Query(ctx, `
				SELECT id, name, image, image_tag, state FROM docker_containers
				WHERE host_id = $1 ORDER BY name`, hostID); cErr == nil {
				defer func() { _ = cRows.Close() }()
				for cRows.Next() {
					var id, name, image, imageTag, state string
					if sErr := cRows.Scan(&id, &name, &image, &imageTag, &state); sErr == nil {
						label := image
						if imageTag != "" && imageTag != "latest" {
							label = image + ":" + imageTag
						}
						opt.Containers = append(opt.Containers, dockerScopeOptionContainer{ID: id, Name: name, Image: label, State: state})
					}
				}
			}

			if pRows, pErr := h.db.Query(ctx, `
				SELECT name, services FROM compose_projects
				WHERE host_id = $1 ORDER BY name`, hostID); pErr == nil {
				defer func() { _ = pRows.Close() }()
				for pRows.Next() {
					var name, servicesJSON string
					if sErr := pRows.Scan(&name, &servicesJSON); sErr == nil {
						var services []string
						_ = json.Unmarshal([]byte(servicesJSON), &services)
						if services == nil {
							services = []string{}
						}
						opt.Projects = append(opt.Projects, dockerScopeOptionProject{Name: name, Services: services})
					}
				}
			}

			hosts = append(hosts, opt)
		}
	}

	c.JSON(http.StatusOK, alertDockerCapabilitiesResponse{
		Metrics: allDockerAlertMetrics(),
		Hosts:   hosts,
	})
}

func allDockerAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "docker_container_not_running", Label: "Container non actif", Unit: "", Icon: "🐳", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "docker_container_running_count", Label: "Containers actifs", Unit: "", Icon: "🐳", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "docker_compose_degraded_services", Label: "Services Compose dégradés", Unit: "", Icon: "🐳", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
	}
}

func (h *AlertRulesHandler) GetAgentAlertRuleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": allAgentAlertMetrics()})
}

// GetSyntheticAlertRuleCapabilities returns metrics exposed by the synthetic
// monitoring workers (uptime probes and SSL certificates). These are global
// metrics — they don't target a specific host.
func (h *AlertRulesHandler) GetSyntheticAlertRuleCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"metrics": allSyntheticAlertMetrics()})
}

func (h *AlertRulesHandler) GetProxmoxAlertRuleCapabilities(c *gin.Context) {
	modes, connections, nodes, storages, guests, disks := h.loadProxmoxScopeOptions(c.Request.Context())
	response := alertSplitCapabilitiesResponse{AgentMetrics: []alertMetricCapability{}, ProxmoxMetrics: allProxmoxAlertMetrics()}
	response.ProxmoxScope.Modes = modes
	response.ProxmoxScope.Connections = connections
	response.ProxmoxScope.Nodes = nodes
	response.ProxmoxScope.Storages = storages
	response.ProxmoxScope.Guests = guests
	response.ProxmoxScope.Disks = disks
	c.JSON(http.StatusOK, response)
}

// allAlertMetrics returns the complete list of all available alert metrics.
func allAgentAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "cpu", Label: "CPU", Unit: "%", Icon: "\u26a1", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "cpu_temperature", Label: "Temp. CPU", Unit: "\u00b0C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "memory", Label: "RAM", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-blue-lt text-blue", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "disk", Label: "Disque", Unit: "%", Icon: "\U0001f4be", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "load", Label: "Load avg", Unit: "", Icon: "\U0001f4c8", BadgeClass: "bg-purple-lt text-purple", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
		{Metric: "heartbeat_timeout", Label: "Heartbeat", Unit: "s", Icon: "\U0001fac0", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "status_offline", Label: "Hote hors ligne", Unit: "", Icon: "\U0001f50c", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_smart_status", Label: "SMART disque", Unit: "", Icon: "\U0001f6e1", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: true},
		{Metric: "disk_temperature", Label: "Temp. disque", Unit: "\u00b0C", Icon: "\U0001f321", BadgeClass: "bg-orange-lt text-orange", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: true},
	}
}

func allProxmoxAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "proxmox_storage_percent", Label: "Proxmox stockage", Unit: "%", Icon: "\U0001f5a5", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_cpu_percent", Label: "Proxmox CPU noeud", Unit: "%", Icon: "\U0001f9e0", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_memory_percent", Label: "Proxmox RAM noeud", Unit: "%", Icon: "\U0001f4ca", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "proxmox_node_cpu_temperature", Label: "Proxmox temp. CPU noeud", Unit: "\u00b0C", Icon: "\U0001f321", BadgeClass: "bg-cyan-lt text-cyan", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
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

func allSyntheticAlertMetrics() []alertMetricCapability {
	return []alertMetricCapability{
		{Metric: "uptime_down_count", Label: "Sondes uptime down", Unit: "", Icon: "\U0001f6a8", BadgeClass: "bg-red-lt text-red", SupportsThreshold: true, SupportsDuration: true, SupportsHostFilter: false},
		{Metric: "ssl_min_days_remaining", Label: "Cert SSL — jours restants", Unit: "j", Icon: "\U0001f510", BadgeClass: "bg-yellow-lt text-yellow", SupportsThreshold: true, SupportsDuration: false, SupportsHostFilter: false},
	}
}

// filterMetricsByCollectors returns only metrics that are available on the host based on its enabled collectors.
// Collectors map example: {"docker": true, "smart": false, "cpu_temp": true, ...}
func filterMetricsByCollectors(allMetrics []alertMetricCapability, collectors map[string]bool) []alertMetricCapability {
	// These metrics are always available (base system metrics)
	alwaysAvailable := map[string]bool{
		"cpu":               true,
		"memory":            true,
		"disk":              true,
		"load":              true,
		"heartbeat_timeout": true,
		"status_offline":    true,
	}

	// These metrics require specific collectors
	requiresCollector := map[string]string{
		"cpu_temperature":   "cpu_temp",
		"disk_smart_status": "smart",
		"disk_temperature":  "smart",
	}

	var filtered []alertMetricCapability
	for _, metric := range allMetrics {
		// Always include base metrics
		if alwaysAvailable[metric.Metric] {
			filtered = append(filtered, metric)
			continue
		}

		// Check if metric requires a specific collector
		if requiredCollector, ok := requiresCollector[metric.Metric]; ok {
			// Check if required collector is enabled
			if collectors[requiredCollector] {
				filtered = append(filtered, metric)
			}
		}
	}

	return filtered
}

// GetHostAlertMetrics returns alert metrics available for a specific host based on its enabled collectors.
func (h *AlertRulesHandler) GetHostAlertMetrics(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostId parameter is required"})
		return
	}

	// Fetch the host to get collectors
	host, err := h.db.GetHost(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}

	// Build response with filtered metrics
	response := alertHostCapabilitiesResponse{
		HostID:   host.ID,
		HostName: host.Name,
		Metrics:  filterMetricsByCollectors(allAgentAlertMetrics(), host.Collectors),
	}

	c.JSON(http.StatusOK, response)
}
