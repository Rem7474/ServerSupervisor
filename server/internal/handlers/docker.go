package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	dockersvc "github.com/serversupervisor/server/internal/services/docker"
)

// DockerHandler translates HTTP to the docker service. Per-host access control
// (requireHostAccess) stays here as it needs the gin context; db is held only for
// that check. The read + dispatch logic lives in internal/services/docker.
type DockerHandler struct {
	svc *dockersvc.Service
	db  *database.DB
}

func NewDockerHandler(svc *dockersvc.Service, db *database.DB) *DockerHandler {
	return &DockerHandler{svc: svc, db: db}
}

// ListContainers returns Docker containers for a specific host.
func (h *DockerHandler) ListContainers(c *gin.Context) {
	containers, err := h.svc.Containers(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
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
	containers, total, err := h.svc.AllContainers(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"containers": containers, "total": total, "limit": limit, "offset": offset})
}

// SendDockerCommand creates a pending docker command for an agent to execute.
func (h *DockerHandler) SendDockerCommand(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	var req models.DockerCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if !requireHostAccess(c, h.db, req.HostID, "operator") {
		return
	}
	id, err := h.svc.SendCommand(c.Request.Context(), req, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": id, "status": "pending"})
}

// ListComposeProjects returns all Docker Compose projects across all hosts.
func (h *DockerHandler) ListComposeProjects(c *gin.Context) {
	projects, err := h.svc.ComposeProjects(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, projects)
}

// ListHostComposeProjects returns Docker Compose projects for a specific host.
func (h *DockerHandler) ListHostComposeProjects(c *gin.Context) {
	projects, err := h.svc.HostComposeProjects(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, projects)
}
