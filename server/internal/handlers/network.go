package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
)

type NetworkHandler struct {
	db *database.DB
}

func NewNetworkHandler(db *database.DB) *NetworkHandler {
	return &NetworkHandler{db: db}
}

func (h *NetworkHandler) GetNetworkSnapshot(c *gin.Context) {
	snapshot, err := networkview.BuildSnapshot(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch network snapshot"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func BuildNetworkSnapshot(db *database.DB) (*models.NetworkSnapshot, error) {
	return networkview.BuildSnapshot(db)
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

// GetTopologySnapshot returns topology with config
func (h *NetworkHandler) GetTopologySnapshot(c *gin.Context) {
	baseSnapshot, err := networkview.BuildSnapshot(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch network snapshot"})
		return
	}

	config, _ := h.db.GetNetworkTopologyConfig()

	snapshot := &models.TopologySnapshot{
		Hosts:      baseSnapshot.Hosts,
		Containers: baseSnapshot.Containers,
		Config:     config,
		UpdatedAt:  time.Now(),
	}

	c.JSON(http.StatusOK, snapshot)
}
