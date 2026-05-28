package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/ws"
)

// isValidWorkingDir returns true when p is either empty, an absolute path,
// and does not escape its root via ".." components.
func isValidWorkingDir(p string) bool {
	if p == "" {
		return true
	}
	if !filepath.IsAbs(p) {
		return false
	}
	return !strings.Contains(filepath.Clean(p), "..")
}

type DockerHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	streamHub  *ws.CommandStreamHub
}

func NewDockerHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, streamHub *ws.CommandStreamHub) *DockerHandler {
	return &DockerHandler{db: db, cfg: cfg, dispatcher: dispatcher, streamHub: streamHub}
}

// ListContainers returns Docker containers for a specific host
func (h *DockerHandler) ListContainers(c *gin.Context) {
	hostID := c.Param("id")
	containers, err := h.db.GetDockerContainers(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}
	if containers == nil {
		containers = []models.DockerContainer{}
	}
	c.JSON(http.StatusOK, containers)
}

// ListAllContainers returns Docker containers across all hosts.
// Accepts optional ?limit (default 500, max 2000) and ?offset query params.
func (h *DockerHandler) ListAllContainers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	containers, err := h.db.GetAllDockerContainers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}
	if containers == nil {
		containers = []models.DockerContainer{}
	}

	total := len(containers)
	end := offset + limit
	if offset >= total {
		containers = []models.DockerContainer{}
	} else {
		if end > total {
			end = total
		}
		containers = containers[offset:end]
	}

	c.JSON(http.StatusOK, gin.H{"containers": containers, "total": total, "limit": limit, "offset": offset})
}

// SendDockerCommand creates a pending docker command for an agent to execute.
func (h *DockerHandler) SendDockerCommand(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req models.DockerCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !requireHostAccess(c, h.db, req.HostID, "operator") {
		return
	}

	// Validate working_dir to prevent path traversal on the agent
	if !isValidWorkingDir(req.WorkingDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid working_dir: must be an absolute path"})
		return
	}

	payload := fmt.Sprintf(`{"working_dir":%q}`, req.WorkingDir)
	result, err := h.dispatcher.Create(c.Request.Context(), dispatch.Request{
		HostID:      req.HostID,
		Module:      "docker",
		Action:      req.Action,
		Target:      req.ContainerName,
		Payload:     payload,
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "docker_" + req.Action,
			HostID:    req.HostID,
			IPAddress: c.ClientIP(),
			Details:   fmt.Sprintf(`{"container":"%s","action":"%s","working_dir":"%s"}`, req.ContainerName, req.Action, req.WorkingDir),
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": result.Command.ID, "status": "pending"})
}

// ListComposeProjects returns all Docker Compose projects across all hosts.
func (h *DockerHandler) ListComposeProjects(c *gin.Context) {
	projects, err := h.db.GetAllComposeProjects(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch compose projects"})
		return
	}
	if projects == nil {
		projects = []models.ComposeProject{}
	}
	c.JSON(http.StatusOK, projects)
}

// ListHostComposeProjects returns Docker Compose projects for a specific host.
func (h *DockerHandler) ListHostComposeProjects(c *gin.Context) {
	hostID := c.Param("id")
	projects, err := h.db.GetComposeProjectsByHost(c.Request.Context(), hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch compose projects"})
		return
	}
	if projects == nil {
		projects = []models.ComposeProject{}
	}
	c.JSON(http.StatusOK, projects)
}
