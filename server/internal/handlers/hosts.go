package handlers

import (
	"net"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type HostHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewHostHandler(db *database.DB, cfg *config.Config) *HostHandler {
	return &HostHandler{db: db, cfg: cfg}
}

// generateAPIKey creates a new API key pair for a host.
// The plain key (returned to the caller) has the format "{hostID}.{secret}".
// The hashed key is a bcrypt hash of the secret and should be stored in the DB.
func generateAPIKey(hostID string) (plainKey, hashedKey string, err error) {
	secret := uuid.New().String()
	hashedKey, err = database.HashAPIKey(secret)
	if err != nil {
		return "", "", err
	}
	return hostID + "." + secret, hashedKey, nil
}

// RegisterHost creates a new host and returns its API key (admin only)
func (h *HostHandler) RegisterHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req models.HostRegistration
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate IP address format
	if net.ParseIP(req.IPAddress) == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid IP address format"})
		return
	}

	hostID := uuid.New().String()
	plainAPIKey, hashedAPIKey, err := generateAPIKey(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	host := &models.Host{
		ID:        hostID,
		Name:      req.Name,
		Hostname:  "", // Will be populated by agent
		IPAddress: req.IPAddress,
		OS:        "", // Will be populated by agent
		APIKey:    hashedAPIKey,
		Tags:      req.Tags,
		Status:    "offline",
	}

	if err := h.db.RegisterHost(host); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register host"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      hostID,
		"api_key": plainAPIKey,
		"message": "Host registered. Use this API key in the agent configuration. It will not be shown again.",
	})
}

// ListHosts returns all hosts
func (h *HostHandler) ListHosts(c *gin.Context) {
	hosts, err := h.db.GetAllHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hosts"})
		return
	}
	if hosts == nil {
		hosts = []models.Host{}
	}
	c.JSON(http.StatusOK, hosts)
}

// GetHost returns a specific host
func (h *HostHandler) GetHost(c *gin.Context) {
	hostID := c.Param("id")
	host, err := h.db.GetHost(hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}
	c.JSON(http.StatusOK, host)
}

// UpdateHost updates editable host fields
func (h *HostHandler) UpdateHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	hostID := c.Param("id")
	var req models.HostUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == nil && req.Hostname == nil && req.IPAddress == nil && req.OS == nil && req.Tags == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if err := h.db.UpdateHost(hostID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update host"})
		return
	}
	updated, err := h.db.GetHost(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch updated host"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteHost removes a host
func (h *HostHandler) DeleteHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	hostID := c.Param("id")
	if err := h.db.DeleteHost(hostID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "host deleted"})
}

// GetHostComplete returns a comprehensive snapshot used for initial page load,
// running all DB queries in parallel to minimise latency.
func (h *HostHandler) GetHostComplete(c *gin.Context) {
	hostID := c.Param("id")

	host, err := h.db.GetHost(hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}

	var (
		metrics     *models.SystemMetrics
		containers  []models.DockerContainer
		aptStatus   *models.AptStatus
		diskMetrics []models.DiskMetrics
		diskHealth  []models.DiskHealth
		cmdHistory  []models.RemoteCommand
	)

	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); metrics, _ = h.db.GetLatestMetrics(hostID) }()
	go func() { defer wg.Done(); containers, _ = h.db.GetDockerContainers(hostID) }()
	go func() { defer wg.Done(); aptStatus, _ = h.db.GetAptStatus(hostID) }()
	go func() { defer wg.Done(); diskMetrics, _ = h.db.GetLatestDiskMetrics(hostID) }()
	go func() { defer wg.Done(); diskHealth, _ = h.db.GetLatestDiskHealth(hostID) }()
	go func() { defer wg.Done(); cmdHistory, _ = h.db.GetRecentCommandsByHost(hostID, 20) }()
	wg.Wait()

	if containers == nil {
		containers = []models.DockerContainer{}
	}
	if diskMetrics == nil {
		diskMetrics = []models.DiskMetrics{}
	}
	if diskHealth == nil {
		diskHealth = []models.DiskHealth{}
	}
	if cmdHistory == nil {
		cmdHistory = []models.RemoteCommand{}
	}

	c.JSON(http.StatusOK, gin.H{
		"host":                 host,
		"metrics":              metrics,
		"containers":           containers,
		"apt_status":           aptStatus,
		"disk_metrics":         diskMetrics,
		"disk_health":          diskHealth,
		"command_history":      cmdHistory,
		"latest_agent_version": LatestAgentVersion,
	})
}

// RotateAPIKey regenerates an API key for a host (admin only)
func (h *HostHandler) RotateAPIKey(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	hostID := c.Param("id")
	plainAPIKey, hashedAPIKey, err := generateAPIKey(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	if err := h.db.UpdateHostAPIKey(hostID, hashedAPIKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key": plainAPIKey,
		"message": "API key rotated. Update the agent configuration immediately; it will not be shown again.",
	})
}

// GetHostDashboard returns complete host info (metrics + docker + apt)
func (h *HostHandler) GetHostDashboard(c *gin.Context) {
	hostID := c.Param("id")

	host, err := h.db.GetHost(hostID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "host not found"})
		return
	}

	metrics, _ := h.db.GetLatestMetrics(hostID)
	containers, _ := h.db.GetDockerContainers(hostID)
	aptStatus, _ := h.db.GetAptStatus(hostID)

	c.JSON(http.StatusOK, gin.H{
		"host":       host,
		"metrics":    metrics,
		"containers": containers,
		"apt_status": aptStatus,
	})
}
