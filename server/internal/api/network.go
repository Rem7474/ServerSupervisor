package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type NetworkHandler struct {
	db *database.DB
}

func NewNetworkHandler(db *database.DB) *NetworkHandler {
	return &NetworkHandler{db: db}
}

func (h *NetworkHandler) GetNetworkSnapshot(c *gin.Context) {
	snapshot, err := buildNetworkSnapshot(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch network snapshot"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func buildNetworkSnapshot(db *database.DB) (*models.NetworkSnapshot, error) {
	hosts, err := db.GetAllHosts()
	if err != nil {
		return nil, err
	}
	containers, err := db.GetAllDockerContainers()
	if err != nil {
		return nil, err
	}

	metricsByHost := make(map[string]*models.SystemMetrics)
	for _, host := range hosts {
		metrics, err := db.GetLatestMetrics(host.ID)
		if err == nil {
			metricsByHost[host.ID] = metrics
		}
	}

	networkHosts := make([]models.NetworkHost, 0, len(hosts))
	for _, host := range hosts {
		rxb := uint64(0)
		txb := uint64(0)
		if metricsByHost[host.ID] != nil {
			rxb = metricsByHost[host.ID].NetworkRxBytes
			txb = metricsByHost[host.ID].NetworkTxBytes
		}
		networkHosts = append(networkHosts, models.NetworkHost{
			ID:             host.ID,
			Name:           host.Name,
			Hostname:       host.Hostname,
			IPAddress:      host.IPAddress,
			Status:         host.Status,
			NetworkRxBytes: rxb,
			NetworkTxBytes: txb,
			LastSeen:       host.LastSeen,
		})
	}

	networkContainers := make([]models.NetworkContainer, 0, len(containers))
	for _, container := range containers {
		mappings := parseDockerPorts(container.Ports)
		networkContainers = append(networkContainers, models.NetworkContainer{
			ID:           container.ID,
			HostID:       container.HostID,
			Hostname:     container.Hostname,
			Name:         container.Name,
			Image:        container.Image,
			ImageTag:     container.ImageTag,
			State:        container.State,
			Status:       container.Status,
			Ports:        container.Ports,
			PortMappings: mappings,
			Labels:       container.Labels,
		})
	}

	return &models.NetworkSnapshot{
		Hosts:      networkHosts,
		Containers: networkContainers,
		UpdatedAt:  time.Now(),
	}, nil
}

func parseDockerPorts(raw string) []models.PortMapping {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []models.PortMapping{}
	}

	parts := strings.Split(trimmed, ",")
	mappings := make([]models.PortMapping, 0, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		mapping := models.PortMapping{Raw: entry}
		if strings.Contains(entry, "->") {
			segments := strings.SplitN(entry, "->", 2)
			hostSide := strings.TrimSpace(segments[0])
			containerSide := strings.TrimSpace(segments[1])

			mapping.HostIP, mapping.HostPort = splitHostBinding(hostSide)
			mapping.ContainerPort, mapping.Protocol = parseContainerPort(containerSide)
		} else {
			mapping.ContainerPort, mapping.Protocol = parseContainerPort(entry)
		}
		mappings = append(mappings, mapping)
	}

	return mappings
}

func splitHostBinding(hostSide string) (string, int) {
	if hostSide == "" {
		return "", 0
	}
	lastColon := strings.LastIndex(hostSide, ":")
	if lastColon == -1 {
		return "", parsePortNumber(hostSide)
	}

	hostIP := strings.TrimSpace(hostSide[:lastColon])
	hostPortStr := strings.TrimSpace(hostSide[lastColon+1:])

	if strings.HasPrefix(hostIP, "[") && strings.HasSuffix(hostIP, "]") {
		hostIP = strings.TrimPrefix(strings.TrimSuffix(hostIP, "]"), "[")
	}
	if hostIP == ":::" {
		hostIP = "::"
	}

	return hostIP, parsePortNumber(hostPortStr)
}

func parseContainerPort(raw string) (int, string) {
	if raw == "" {
		return 0, ""
	}
	parts := strings.SplitN(raw, "/", 2)
	port := parsePortNumber(parts[0])
	proto := ""
	if len(parts) > 1 {
		proto = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	return port, proto
}

func parsePortNumber(raw string) int {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return 0
	}
	if strings.Contains(clean, "-") {
		clean = strings.SplitN(clean, "-", 2)[0]
	}
	value, err := strconv.Atoi(clean)
	if err != nil {
		return 0
	}
	return value
}

// GetTopologyConfig returns persisted network configuration
func (h *NetworkHandler) GetTopologyConfig(c *gin.Context) {
	cfg, err := h.db.GetNetworkTopologyConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// SaveTopologyConfig persists network configuration
func (h *NetworkHandler) SaveTopologyConfig(c *gin.Context) {
	var cfg models.NetworkTopologyConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.SaveNetworkTopologyConfig(&cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetTopologySnapshot returns enhanced topology with inferred links
func (h *NetworkHandler) GetTopologySnapshot(c *gin.Context) {
	// Get base network snapshot
	baseSnapshot, err := buildNetworkSnapshot(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch network snapshot"})
		return
	}

	// Get Docker networks
	networks, _ := h.db.GetAllDockerNetworks()

	// Get persisted config
	config, _ := h.db.GetNetworkTopologyConfig()

	// Infer topology links
	links := inferTopologyLinks(h.db, baseSnapshot.Containers, networks)

	// Build topology snapshot
	snapshot := &models.TopologySnapshot{
		Hosts:      baseSnapshot.Hosts,
		Containers: baseSnapshot.Containers,
		Networks:   networks,
		Links:      links,
		Config:     config,
		UpdatedAt:  time.Now(),
	}

	c.JSON(http.StatusOK, snapshot)
}

// inferTopologyLinks infers logical links between containers based on 3 rules:
// 1. Docker networks (shared network = connection)
// 2. Environment variables (*_HOST, *_URL, *_ADDR patterns)
// 3. Labels Traefik (proxy detection)
func inferTopologyLinks(db *database.DB, containers []models.NetworkContainer, networks []models.DockerNetwork) []models.TopologyLink {
	var links []models.TopologyLink

	// Build container lookup map for Rule 2 (env vars)
	containerByName := make(map[string]models.NetworkContainer)
	for _, c := range containers {
		containerByName[c.Name] = c
	}

	// Rule 1: Docker networks - star topology to avoid O(n²) explosion
	// Only create links for custom networks (skip bridge/host/none)
	// network.ContainerIDs contains container NAMES (not IDs) due to docker network inspect format
	// Link first container (anchor) to others, max 8 links per network
	for _, network := range networks {
		if len(network.ContainerIDs) < 2 {
			continue
		}
		// Ignore generic system networks
		if network.Name == "bridge" || network.Name == "host" || network.Name == "none" {
			continue
		}
		// Star topology: link anchor (first container name) to others
		maxLinks := 8
		linkCount := 0
		anchorName := network.ContainerIDs[0]
		anchor, okAnchor := containerByName[anchorName]
		if !okAnchor || anchor.Name == "" {
			continue
		}
		for _, targetName := range network.ContainerIDs[1:] {
			if linkCount >= maxLinks {
				break
			}
			target, okTarget := containerByName[targetName]
			if !okTarget || target.Name == "" {
				continue
			}
			links = append(links, models.TopologyLink{
				SourceContainerName: anchor.Name,
				SourceHostID:        anchor.HostID,
				TargetContainerName: target.Name,
				TargetHostID:        target.HostID,
				LinkType:            "network",
				NetworkName:         network.Name,
				Confidence:          70,
			})
			linkCount++
		}
	}

	// Rule 2: Environment variables referencing other containers
	envs, errEnvs := db.GetAllContainerEnvs()
	if errEnvs == nil && len(envs) > 0 {
		for _, env := range envs {
			source, okSource := containerByName[env.ContainerName]
			if !okSource || source.Name == "" {
				continue
			}
			// Check each env var for patterns like DB_HOST, REDIS_URL, etc.
			for key, value := range env.EnvVars {
				if !isHostLikeVar(key) {
					continue
				}
				// Extract hostname/container name from value
				targetName := extractContainerNameFromEnv(value)
				if targetName == "" {
					continue
				}
				target, okTarget := containerByName[targetName]
				if !okTarget || target.Name == "" {
					continue
				}
				links = append(links, models.TopologyLink{
					SourceContainerName: source.Name,
					SourceHostID:        source.HostID,
					TargetContainerName: target.Name,
					TargetHostID:        target.HostID,
					LinkType:            "env_ref",
					EnvKey:              key,
					Confidence:          90,
				})
			}
		}
	}

	// Rule 3: Proxy detection - only link proxy to containers on the same Docker network
	// Build a set of networks per container name for fast lookup
	containerNetworks := make(map[string]map[string]bool) // containerName -> set of networkIDs
	for _, network := range networks {
		for _, cName := range network.ContainerIDs {
			if containerNetworks[cName] == nil {
				containerNetworks[cName] = make(map[string]bool)
			}
			containerNetworks[cName][network.ID] = true
		}
	}

	for _, c := range containers {
		// Helper to detect proxy containers with precision
		if !isProxyContainer(c) {
			continue
		}
		proxyNetworks := containerNetworks[c.Name]
		if len(proxyNetworks) == 0 {
			continue // proxy not on any known network, skip
		}
		for _, target := range containers {
			if target.ID == c.ID {
				continue
			}
			// Only link if they share at least one Docker network
			targetNetworks := containerNetworks[target.Name]
			sharedNetwork := false
			for netID := range proxyNetworks {
				if targetNetworks[netID] {
					sharedNetwork = true
					break
				}
			}
			if !sharedNetwork {
				continue
			}
			// Only link if target has published ports (likely serves traffic)
			if len(target.PortMappings) == 0 {
				continue
			}
			links = append(links, models.TopologyLink{
				SourceContainerName: c.Name,
				SourceHostID:        c.HostID,
				TargetContainerName: target.Name,
				TargetHostID:        target.HostID,
				LinkType:            "proxy",
				Confidence:          75,
			})
		}
	}

	// Deduplicate links
	return deduplicateLinks(links)
}

// isHostLikeVar checks if an env var name suggests a hostname/connection
func isHostLikeVar(key string) bool {
	upperKey := strings.ToUpper(key)
	patterns := []string{"_HOST", "_URL", "_ADDR", "_ENDPOINT", "_SERVICE", "_DATABASE_URL", "_CONNECTION", "_SERVER"}
	for _, pattern := range patterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}
	return false
}

// extractContainerNameFromEnv extracts a container name from an env var value
func extractContainerNameFromEnv(value string) string {
	// Remove common protocols
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "postgres://")
	value = strings.TrimPrefix(value, "mysql://")
	value = strings.TrimPrefix(value, "redis://")

	// Remove port number (everything after :)
	if idx := strings.Index(value, ":"); idx > 0 {
		value = value[:idx]
	}

	// Remove path (everything after /)
	if idx := strings.Index(value, "/"); idx > 0 {
		value = value[:idx]
	}

	// Remove query string
	if idx := strings.Index(value, "?"); idx > 0 {
		value = value[:idx]
	}

	value = strings.TrimSpace(value)
	if len(value) > 2 {
		return value
	}
	return ""
}

// isProxyContainer detects proxy containers with precision (nginx, traefik, caddy, etc)
func isProxyContainer(c models.NetworkContainer) bool {
	// Only running containers can be proxies
	if c.State != "running" {
		return false
	}

	imageLower := strings.ToLower(c.Image)

	// Extract image name without registry and tag
	imageName := imageLower
	if idx := strings.LastIndex(imageLower, "/"); idx >= 0 {
		imageName = imageLower[idx+1:]
	}
	if idx := strings.LastIndex(imageName, ":"); idx >= 0 {
		imageName = imageName[:idx]
	}

	// Known proxy image names (exact match)
	exactProxies := []string{"nginx", "traefik", "caddy", "haproxy", "nginx-proxy-manager"}
	for _, p := range exactProxies {
		if imageName == p {
			return true
		}
	}

	// Match common proxy image variants (with registry)
	proxyPatterns := []string{
		"jc21/nginx-proxy-manager",
		"traefik:",
		"/nginx:",
		":nginx",
	}
	for _, pattern := range proxyPatterns {
		if strings.Contains(imageLower, pattern) {
			return true
		}
	}

	return false
}

// deduplicateLinks removes duplicate links
func deduplicateLinks(links []models.TopologyLink) []models.TopologyLink {
	seen := make(map[string]bool)
	var result []models.TopologyLink
	for _, link := range links {
		// Create a key based on source, target, and type
		key := link.SourceContainerName + "|" + link.TargetContainerName + "|" + link.LinkType
		if !seen[key] {
			seen[key] = true
			result = append(result, link)
		}
	}
	return result
}
