package networkview

import (
	"strconv"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

func BuildSnapshot(db *database.DB) (*models.NetworkSnapshot, error) {
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
			NetRxBytes:   container.NetRxBytes,
			NetTxBytes:   container.NetTxBytes,
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
	hostSide = strings.TrimSpace(hostSide)
	if hostSide == "" {
		return "", 0
	}

	if strings.Count(hostSide, ":") == 0 {
		return "", parsePortNumber(hostSide)
	}

	parts := strings.Split(hostSide, ":")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), parsePortNumber(parts[1])
	}

	hostPort := parts[len(parts)-1]
	hostIP := strings.Join(parts[:len(parts)-1], ":")
	return strings.TrimSpace(hostIP), parsePortNumber(hostPort)
}

func parseContainerPort(raw string) (int, string) {
	parts := strings.SplitN(strings.TrimSpace(raw), "/", 2)
	port := parsePortNumber(parts[0])
	protocol := "tcp"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		protocol = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	return port, protocol
}

func parsePortNumber(raw string) int {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "[]")
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		value = value[idx+1:]
	}
	port, _ := strconv.Atoi(value)
	return port
}
