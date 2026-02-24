package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type DockerHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *AptStreamHub
}

func NewDockerHandler(db *database.DB, cfg *config.Config, streamHub *AptStreamHub) *DockerHandler {
	return &DockerHandler{db: db, cfg: cfg, streamHub: streamHub}
}

// ListContainers returns Docker containers for a specific host
func (h *DockerHandler) ListContainers(c *gin.Context) {
	hostID := c.Param("id")
	containers, err := h.db.GetDockerContainers(hostID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}
	if containers == nil {
		containers = []models.DockerContainer{}
	}
	c.JSON(http.StatusOK, containers)
}

// ListAllContainers returns all Docker containers across all hosts
func (h *DockerHandler) ListAllContainers(c *gin.Context) {
	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}
	if containers == nil {
		containers = []models.DockerContainer{}
	}
	c.JSON(http.StatusOK, containers)
}

// CompareVersions compares running docker images with tracked GitHub releases
func (h *DockerHandler) CompareVersions(c *gin.Context) {
	repos, err := h.db.GetTrackedRepos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch tracked repos"})
		return
	}

	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch containers"})
		return
	}

	hosts, err := h.db.GetAllHosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hosts"})
		return
	}
	hostMap := make(map[string]string)
	for _, h := range hosts {
		hostMap[h.ID] = h.Hostname
	}

	var comparisons []models.VersionComparison
	for _, repo := range repos {
		if repo.DockerImage == "" {
			continue
		}
		for _, container := range containers {
			if container.Image == repo.DockerImage || container.Image+":"+container.ImageTag == repo.DockerImage {
				comparisons = append(comparisons, models.VersionComparison{
					DockerImage:    container.Image,
					RunningVersion: container.ImageTag,
					LatestVersion:  repo.LatestVersion,
					IsUpToDate:     normalizeVersion(container.ImageTag) == normalizeVersion(repo.LatestVersion),
					RepoOwner:      repo.Owner,
					RepoName:       repo.Repo,
					ReleaseURL:     repo.ReleaseURL,
					HostID:         container.HostID,
					Hostname:       hostMap[container.HostID],
				})
			}
		}
	}
	c.JSON(http.StatusOK, comparisons)
}

// TrackedRepos management

func (h *DockerHandler) ListTrackedRepos(c *gin.Context) {
	repos, err := h.db.GetTrackedRepos()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch repos"})
		return
	}
	if repos == nil {
		repos = []models.TrackedRepo{}
	}
	c.JSON(http.StatusOK, repos)
}

func (h *DockerHandler) AddTrackedRepo(c *gin.Context) {
	var req models.TrackedRepoCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo := &models.TrackedRepo{
		Owner:       req.Owner,
		Repo:        req.Repo,
		DisplayName: req.DisplayName,
		DockerImage: req.DockerImage,
	}
	if repo.DisplayName == "" {
		repo.DisplayName = req.Owner + "/" + req.Repo
	}

	if err := h.db.CreateTrackedRepo(repo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tracked repo"})
		return
	}
	c.JSON(http.StatusCreated, repo)
}

func (h *DockerHandler) DeleteTrackedRepo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.db.DeleteTrackedRepo(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete repo"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "repo deleted"})
}

// SendDockerCommand creates a pending docker command for an agent to execute
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

	// Create audit log
	details := fmt.Sprintf(`{"container":"%s","action":"%s"}`, req.ContainerName, req.Action)
	auditID, auditErr := h.db.CreateAuditLog(username, "docker_"+req.Action, req.HostID, c.ClientIP(), details, "pending")
	var auditLogIDPtr *int64
	if auditErr != nil {
		log.Printf("Warning: failed to create audit log for docker command: %v", auditErr)
	} else {
		auditLogIDPtr = &auditID
	}

	cmd, err := h.db.CreateDockerCommand(req.HostID, req.ContainerName, req.Action, username, auditLogIDPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"command_id": cmd.ID, "status": "pending"})
}

// normalizeVersion strips leading 'v' from version strings for comparison
func normalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}
