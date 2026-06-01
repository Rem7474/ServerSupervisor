package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
)

// ========== HTTP handlers ==========

func (h *ReleaseTrackerHandler) List(c *gin.Context) {
	trackers, err := h.db.ListReleaseTrackers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trackers"})
		return
	}
	if trackers == nil {
		trackers = []models.ReleaseTracker{}
	}
	c.JSON(http.StatusOK, gin.H{"trackers": trackers})
}

func (h *ReleaseTrackerHandler) Create(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}

	var req models.ReleaseTracker
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TrackerType == "" {
		req.TrackerType = "git"
	}
	if req.TrackerType != "git" && req.TrackerType != "docker" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tracker_type must be 'git' or 'docker'"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.CooldownHours < 0 || req.CooldownHours > 168 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cooldown_hours must be between 0 and 168"})
		return
	}

	if req.TrackerType == "git" {
		// Git trackers can run in monitor-only mode (no host/task),
		// but host/task must be provided together when dispatch is enabled.
		if (req.HostID == "") != (req.CustomTaskID == "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host_id and custom_task_id must be provided together for git trackers"})
			return
		}
		if req.RepoOwner == "" || req.RepoName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repo_owner and repo_name are required for git trackers"})
			return
		}
		if req.Provider == "" {
			req.Provider = "github"
		}
		if !validReleaseProviders[req.Provider] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider; must be github, gitlab, or gitea"})
			return
		}
	} else { // docker
		if status, msg := validateDockerTracker(&req); status != 0 {
			c.JSON(status, gin.H{"error": msg})
			return
		}
	}

	if req.NotifyChannels == nil {
		req.NotifyChannels = []string{}
	}

	created, err := h.db.CreateReleaseTracker(c.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), fmt.Sprintf("CreateReleaseTracker: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tracker"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"tracker": created})
}

// ListTrackableContainers returns compose-managed containers without a tracker,
// for the auto-discovery UI ("Conteneurs détectés").
func (h *ReleaseTrackerHandler) ListTrackableContainers(c *gin.Context) {
	containers, err := h.db.ListTrackableContainers(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), fmt.Sprintf("ListTrackableContainers: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list trackable containers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

// CreateBulk creates multiple compose trackers in one call (auto-discovery).
// Each entry is validated independently; the response reports per-entry results
// so a few invalid rows do not abort the whole batch.
func (h *ReleaseTrackerHandler) CreateBulk(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}

	var req struct {
		Trackers []models.ReleaseTracker `json:"trackers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Trackers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trackers array is required"})
		return
	}
	if len(req.Trackers) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many trackers (max 100)"})
		return
	}

	type bulkResult struct {
		Name    string `json:"name"`
		Created bool   `json:"created"`
		Error   string `json:"error,omitempty"`
	}
	results := make([]bulkResult, 0, len(req.Trackers))
	createdCount := 0

	for _, t := range req.Trackers {
		t.TrackerType = "docker"
		if t.Name == "" {
			results = append(results, bulkResult{Name: t.Name, Error: "name is required"})
			continue
		}
		if t.CooldownHours < 0 || t.CooldownHours > 168 {
			results = append(results, bulkResult{Name: t.Name, Error: "cooldown_hours must be between 0 and 168"})
			continue
		}
		if status, msg := validateDockerTracker(&t); status != 0 {
			results = append(results, bulkResult{Name: t.Name, Error: msg})
			continue
		}
		if t.NotifyChannels == nil {
			t.NotifyChannels = []string{}
		}
		if _, err := h.db.CreateReleaseTracker(c.Request.Context(), t); err != nil {
			slog.ErrorContext(c.Request.Context(), fmt.Sprintf("CreateBulk: failed to create %q: %v", t.Name, err))
			results = append(results, bulkResult{Name: t.Name, Error: "failed to create"})
			continue
		}
		createdCount++
		results = append(results, bulkResult{Name: t.Name, Created: true})
	}

	c.JSON(http.StatusOK, gin.H{"created": createdCount, "results": results})
}

func (h *ReleaseTrackerHandler) Get(c *gin.Context) {
	id := c.Param("id")
	t, err := h.db.GetReleaseTrackerByID(c.Request.Context(), id)
	if err == sql.ErrNoRows {
		slog.InfoContext(c.Request.Context(), fmt.Sprintf("Release tracker history: tracker not found (id=%s)", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), fmt.Sprintf("Release tracker history: failed to load tracker (id=%s): %v", id, err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}
	execs, _ := h.db.ListReleaseTrackerExecutions(c.Request.Context(), id, 20)
	c.JSON(http.StatusOK, gin.H{"tracker": t, "executions": execs})
}

func (h *ReleaseTrackerHandler) Update(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")

	var req models.ReleaseTracker
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TrackerType == "" {
		req.TrackerType = "git"
	}
	if req.TrackerType != "git" && req.TrackerType != "docker" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tracker_type must be 'git' or 'docker'"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.CooldownHours < 0 || req.CooldownHours > 168 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cooldown_hours must be between 0 and 168"})
		return
	}
	if req.TrackerType == "git" {
		if (req.HostID == "") != (req.CustomTaskID == "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host_id and custom_task_id must be provided together for git trackers"})
			return
		}
		if req.RepoOwner == "" || req.RepoName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repo_owner and repo_name are required for git trackers"})
			return
		}
		if !validReleaseProviders[req.Provider] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
			return
		}
	} else { // docker
		if status, msg := validateDockerTracker(&req); status != 0 {
			c.JSON(status, gin.H{"error": msg})
			return
		}
	}

	if err := h.db.UpdateReleaseTracker(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tracker"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ReleaseTrackerHandler) Delete(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")
	if err := h.db.DeleteReleaseTracker(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tracker"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *ReleaseTrackerHandler) TriggerCheck(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")

	t, err := h.db.GetReleaseTrackerByID(c.Request.Context(), id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}

	go h.checkOne(h.pollerCtx, *t)
	c.JSON(http.StatusOK, gin.H{"status": "check scheduled"})
}

// Run manually triggers the tracker's custom task with the last known release info.
func (h *ReleaseTrackerHandler) Run(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin or operator role required"})
		return
	}
	id := c.Param("id")

	t, err := h.db.GetReleaseTrackerByID(c.Request.Context(), id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}

	if t.TrackerType == "docker" {
		if t.UpdateAction == "compose" && !trackerHasDispatchTarget(*t) {
			c.JSON(http.StatusConflict, gin.H{"error": "mode compose : configurez une VM cible et un projet compose pour déclencher manuellement"})
			return
		}
		if t.LatestImageDigest == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "aucune vérification initiale effectuée — attendez le prochain cycle de polling avant de déclencher manuellement"})
			return
		}
		tag := t.DockerTag
		if tag == "" {
			tag = "latest"
		}
		go h.dispatchDockerTracker(h.pollerCtx, *t, tag, t.LastReleaseTag, t.LatestImageDigest, t.LatestImageDigest)
	} else {
		if t.HostID == "" || t.CustomTaskID == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "tracker en mode surveillance seule — configurez une VM cible et une tâche pour déclencher manuellement"})
			return
		}
		if t.LastReleaseTag == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "aucune release initiale enregistrée — attendez le prochain cycle de polling avant de déclencher manuellement"})
			return
		}
		go h.dispatchGitRelease(h.pollerCtx, *t, t.LastReleaseTag, "", "")
	}
	c.JSON(http.StatusOK, gin.H{"status": "execution scheduled"})
}

func (h *ReleaseTrackerHandler) GetExecutions(c *gin.Context) {
	id := c.Param("id")
	execs, err := h.db.ListReleaseTrackerExecutions(c.Request.Context(), id, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list executions"})
		return
	}
	if execs == nil {
		execs = []models.ReleaseTrackerExecution{}
	}
	c.JSON(http.StatusOK, gin.H{"executions": execs})
}

func (h *ReleaseTrackerHandler) GetVersionHistory(c *gin.Context) {
	id := c.Param("id")

	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > 50 {
				n = 50
			}
			limit = n
		}
	}

	t, err := h.db.GetReleaseTrackerByID(c.Request.Context(), id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}

	history := make([]models.ReleaseVersionHistoryItem, 0)
	if t.TrackerType == "docker" {
		history, err = h.db.ListTrackerTagDigests(c.Request.Context(), id, limit)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), fmt.Sprintf("Release tracker history: docker history load error (tracker=%s id=%s image=%s tag=%s limit=%d): %v", t.Name, t.ID, t.DockerImage, t.DockerTag, limit, err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load docker version history"})
			return
		}
	} else {
		providerClient := gitprovider.NewClient(t.Provider, h.cfg.GitHubToken)
		releases, ferr := providerClient.FetchReleaseHistory(t.RepoOwner, t.RepoName, limit)
		if ferr != nil {
			slog.ErrorContext(c.Request.Context(), fmt.Sprintf("Release tracker history: provider call failed (tracker=%s id=%s provider=%s repo=%s/%s limit=%d): %v", t.Name, t.ID, t.Provider, t.RepoOwner, t.RepoName, limit, ferr))
			c.JSON(http.StatusBadGateway, gin.H{"error": ferr.Error()})
			return
		}
		for _, r := range releases {
			item := models.ReleaseVersionHistoryItem{
				Version:    r.TagName,
				Name:       r.Name,
				ReleaseURL: r.HTMLURL,
			}
			if !r.PublishedAt.IsZero() {
				published := r.PublishedAt
				item.PublishedAt = &published
			}
			history = append(history, item)
		}
	}

	if history == nil {
		history = []models.ReleaseVersionHistoryItem{}
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}
