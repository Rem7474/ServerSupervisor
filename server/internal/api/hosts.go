package api

import (
	"net"
	"net/http"

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

// RegisterHost creates a new host and returns its API key (admin only)
func (h *HostHandler) RegisterHost(c *gin.Context) {
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
	plainAPIKey := uuid.New().String()
	hashedAPIKey := database.HashAPIKey(plainAPIKey)

	host := &models.Host{
		ID:        hostID,
		Name:      req.Name,
		Hostname:  "", // Will be populated by agent
		IPAddress: req.IPAddress,
		OS:        "", // Will be populated by agent
		APIKey:    hashedAPIKey,
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
	hostID := c.Param("id")
	var req models.HostUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == nil && req.Hostname == nil && req.IPAddress == nil && req.OS == nil {
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
	hostID := c.Param("id")
	if err := h.db.DeleteHost(hostID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete host"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "host deleted"})
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
