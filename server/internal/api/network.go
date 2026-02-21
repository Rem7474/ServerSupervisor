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
