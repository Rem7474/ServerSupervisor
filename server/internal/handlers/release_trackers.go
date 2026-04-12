package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/ws"
)

var validReleaseProviders = map[string]bool{
	"github": true, "gitlab": true, "gitea": true,
}

type ReleaseTrackerHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	notifHub   *ws.NotificationHub
	stop       chan struct{}
}

func NewReleaseTrackerHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *ReleaseTrackerHandler {
	return &ReleaseTrackerHandler{
		db:         db,
		cfg:        cfg,
		dispatcher: dispatcher,
		notifHub:   notifHub,
		stop:       make(chan struct{}),
	}
}

// StartPoller begins periodic polling of release trackers.
func (h *ReleaseTrackerHandler) StartPoller() {
	interval := h.cfg.GitHubPollInterval
	if interval == 0 {
		interval = 15 * time.Minute
	}
	log.Printf("Release tracker poller started (interval: %v)", interval)

	go h.checkAll()

	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				h.checkAll()
			case <-h.stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (h *ReleaseTrackerHandler) StopPoller() {
	close(h.stop)
}

func (h *ReleaseTrackerHandler) checkAll() {
	trackers, err := h.db.GetEnabledReleaseTrackers()
	if err != nil {
		log.Printf("Release tracker poller: failed to fetch trackers: %v", err)
		return
	}
	for _, t := range trackers {
		h.checkOne(t)
	}
}

func (h *ReleaseTrackerHandler) checkOne(t models.ReleaseTracker) {
	switch t.TrackerType {
	case "docker":
		h.checkOneDocker(t)
	default: // "git"
		h.checkOneGit(t)
	}
}

// checkOneGit polls a git provider for new releases.
func (h *ReleaseTrackerHandler) checkOneGit(t models.ReleaseTracker) {
	providerClient := gitprovider.NewClient(t.Provider, h.cfg.GitHubToken)
	tag, releaseURL, releaseName, err := providerClient.FetchLatestRelease(t.RepoOwner, t.RepoName)
	if err != nil {
		log.Printf("Git tracker %s (%s/%s): fetch error: %v", t.Name, t.RepoOwner, t.RepoName, err)
		_ = h.db.UpdateReleaseTrackerError(t.ID, err.Error())
		return
	}

	if tag == "" {
		_ = h.db.UpdateReleaseTrackerError(t.ID, "aucune release ou tag trouvé sur ce dépôt")
		return
	}

	// First check — just record the current tag without triggering
	if t.LastReleaseTag == "" {
		log.Printf("Git tracker %s: initial tag recorded: %s", t.Name, tag)
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, tag, false)
		return
	}

	// No change
	if tag == t.LastReleaseTag {
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, "", false)
		return
	}

	// New release detected
	log.Printf("Git tracker %s: new release %s (was %s) → dispatching task %s on host %s",
		t.Name, tag, t.LastReleaseTag, t.CustomTaskID, t.HostID)

	h.dispatchGitRelease(t, tag, releaseURL, releaseName)
}

// checkOneDocker polls the Docker registry for a new image digest.
func (h *ReleaseTrackerHandler) checkOneDocker(t models.ReleaseTracker) {
	if t.DockerImage == "" {
		_ = h.db.UpdateReleaseTrackerError(t.ID, "docker_image manquant")
		return
	}
	tag := t.DockerTag
	if tag == "" {
		tag = "latest"
	}

	providerClient := gitprovider.NewClient("github", h.cfg.GitHubToken)
	digest, err := providerClient.FetchDockerManifestDigest(t.DockerImage, tag)
	if err != nil {
		log.Printf("Docker tracker %s (%s:%s): fetch error: %v", t.Name, t.DockerImage, tag, err)
		_ = h.db.UpdateReleaseTrackerError(t.ID, err.Error())
		return
	}
	if digest == "" {
		_ = h.db.UpdateReleaseTrackerError(t.ID, "digest vide retourné par le registre")
		return
	}

	// For "latest" tags, attempt to resolve the actual version from the registry.
	// Docker Hub: uses hub.docker.com API (tags + digest in one call).
	// Others: enumerates semver-looking tags and HEADs each manifest.
	resolvedVersion := tag
	if tag == "latest" {
		if v := providerClient.FetchDockerVersionForDigest(t.DockerImage, digest); v != "" {
			resolvedVersion = v
			log.Printf("Docker tracker %s: resolved 'latest' → %s", t.Name, v)
		}
	}

	// First check — record current digest without triggering
	if t.LatestImageDigest == "" {
		log.Printf("Docker tracker %s: initial digest recorded for %s:%s", t.Name, t.DockerImage, tag)
		_ = h.db.UpdateReleaseTrackerDigest(t.ID, digest)
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, resolvedVersion, false)
		if resolvedVersion != tag {
			_ = h.db.StoreTrackerTagDigest(t.ID, resolvedVersion, digest)
		}
		return
	}

	// No change
	if digest == t.LatestImageDigest {
		// If we previously stored "latest" as the tag but can now resolve a version, update it.
		if resolvedVersion != tag && t.LastReleaseTag == tag {
			_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, resolvedVersion, false)
			_ = h.db.StoreTrackerTagDigest(t.ID, resolvedVersion, digest)
		} else {
			_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, "", false)
		}
		return
	}

	// New image detected
	oldDigest := t.LatestImageDigest
	_ = h.db.UpdateReleaseTrackerDigest(t.ID, digest)
	if resolvedVersion != tag {
		_ = h.db.StoreTrackerTagDigest(t.ID, resolvedVersion, digest)
	}

	// Monitor-only mode: no task configured, just record the new version.
	if t.CustomTaskID == "" || t.HostID == "" {
		log.Printf("Docker tracker %s: new digest for %s:%s (monitor-only, no task dispatched)", t.Name, t.DockerImage, tag)
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, resolvedVersion, false)
		return
	}

	log.Printf("Docker tracker %s: new digest for %s:%s → dispatching task %s on host %s",
		t.Name, t.DockerImage, tag, t.CustomTaskID, t.HostID)
	h.dispatchDockerUpdate(t, tag, resolvedVersion, oldDigest, digest)
}

func (h *ReleaseTrackerHandler) dispatchGitRelease(t models.ReleaseTracker, tag, releaseURL, releaseName string) {
	// Skip if already running
	running, _ := h.db.GetRunningExecutionForReleaseTracker(t.ID)
	if running {
		log.Printf("Git tracker %s: skipping dispatch (already running)", t.Name)
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, tag, false)
		return
	}

	exec, err := h.db.CreateReleaseTrackerExecution(models.ReleaseTrackerExecution{
		TrackerID:   t.ID,
		TagName:     tag,
		ReleaseURL:  releaseURL,
		ReleaseName: releaseName,
		Status:      "pending",
	})
	if err != nil {
		log.Printf("Git tracker %s: failed to create execution: %v", t.Name, err)
		return
	}

	envVars := map[string]string{
		"SS_REPO_NAME":    t.RepoOwner + "/" + t.RepoName,
		"SS_TAG_NAME":     tag,
		"SS_RELEASE_URL":  releaseURL,
		"SS_RELEASE_NAME": releaseName,
		"SS_TRACKER_NAME": t.Name,
	}
	envPayload, _ := json.Marshal(map[string]interface{}{"env": envVars})

	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := h.dispatcher.Create(dispatch.Request{
		HostID:      t.HostID,
		Module:      "custom",
		Action:      "run",
		Target:      t.CustomTaskID,
		Payload:     string(envPayload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "release_trigger",
			HostID:    t.HostID,
			IPAddress: "poller",
			Details:   fmt.Sprintf(`{"tracker_id":%q,"repo":%q,"tag":%q}`, t.ID, t.RepoOwner+"/"+t.RepoName, tag),
		},
	})
	if err != nil {
		log.Printf("Git tracker %s: failed to create command: %v", t.Name, err)
		now := time.Now()
		_ = h.db.UpdateReleaseTrackerExecutionStatus(exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(exec.ID, result.Command.ID)
	_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, tag, true)
}

func (h *ReleaseTrackerHandler) dispatchDockerUpdate(t models.ReleaseTracker, tag, resolvedVersion, oldDigest, newDigest string) {
	running, _ := h.db.GetRunningExecutionForReleaseTracker(t.ID)
	if running {
		log.Printf("Docker tracker %s: skipping dispatch (already running)", t.Name)
		_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, resolvedVersion, false)
		return
	}

	imageFull := t.DockerImage + ":" + tag
	exec, err := h.db.CreateReleaseTrackerExecution(models.ReleaseTrackerExecution{
		TrackerID:   t.ID,
		TagName:     resolvedVersion,
		ReleaseURL:  "",
		ReleaseName: imageFull,
		Status:      "pending",
	})
	if err != nil {
		log.Printf("Docker tracker %s: failed to create execution: %v", t.Name, err)
		return
	}

	envVars := map[string]string{
		"SS_IMAGE_NAME":    imageFull,
		"SS_IMAGE_TAG":     tag,
		"SS_IMAGE_VERSION": resolvedVersion, // actual version ("1.25.3") or same as tag if unresolved
		"SS_OLD_DIGEST":    oldDigest,
		"SS_NEW_DIGEST":    newDigest,
		"SS_TRACKER_NAME":  t.Name,
	}
	envPayload, _ := json.Marshal(map[string]interface{}{"env": envVars})

	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := h.dispatcher.Create(dispatch.Request{
		HostID:      t.HostID,
		Module:      "custom",
		Action:      "run",
		Target:      t.CustomTaskID,
		Payload:     string(envPayload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "docker_tracker_trigger",
			HostID:    t.HostID,
			IPAddress: "poller",
			Details:   fmt.Sprintf(`{"tracker_id":%q,"image":%q,"digest":%q}`, t.ID, imageFull, newDigest),
		},
	})
	if err != nil {
		log.Printf("Docker tracker %s: failed to create command: %v", t.Name, err)
		now := time.Now()
		_ = h.db.UpdateReleaseTrackerExecutionStatus(exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(exec.ID, result.Command.ID)
	_ = h.db.UpdateReleaseTrackerLastSeen(t.ID, resolvedVersion, true)
}

// NotifyComplete is called by the agent handler when a command completes.
func (h *ReleaseTrackerHandler) NotifyComplete(commandID, status string) {
	trackerID, notifyOnRelease, channels, err := h.db.UpdateReleaseTrackerExecutionByCommandID(commandID, status)
	if err != nil {
		return // not a tracker command
	}

	if !notifyOnRelease || len(channels) == 0 {
		return
	}

	tracker, err := h.db.GetReleaseTrackerByID(trackerID)
	if err != nil {
		return
	}

	emoji := "✅"
	if status == "failed" {
		emoji = "❌"
	}

	var subject, msg string
	if tracker.TrackerType == "docker" {
		imageFull := tracker.DockerImage + ":" + tracker.DockerTag
		if tracker.DockerTag == "" {
			imageFull = tracker.DockerImage + ":latest"
		}
		subject = fmt.Sprintf("[ServerSupervisor] Docker tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Docker tracker '%s' (%s) execution %s on host %s (task: %s)",
			tracker.Name, imageFull, status, tracker.HostID, tracker.CustomTaskID)
	} else {
		subject = fmt.Sprintf("[ServerSupervisor] Release tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Release tracker '%s' (%s/%s) execution %s on host %s (task: %s)",
			tracker.Name, tracker.RepoOwner, tracker.RepoName, status, tracker.HostID, tracker.CustomTaskID)
	}

	notifier := notify.New()
	for _, ch := range channels {
		switch ch {
		case "smtp":
			to := h.cfg.SMTPTo
			if to == "" || h.cfg.SMTPFrom == "" {
				continue
			}
			if err := notifier.SendSMTP(h.cfg, h.cfg.SMTPFrom, to, subject, msg); err != nil {
				log.Printf("Release tracker SMTP send: %v", err)
			}

		case "ntfy":
			ntfyURL := h.cfg.NotifyURL
			if ntfyURL == "" {
				continue
			}
			if err := notifier.SendNtfy(h.cfg, ntfyURL, subject, msg); err != nil {
				log.Printf("Release tracker notify ntfy: %v", err)
			}

		case "browser":
			if h.notifHub == nil {
				continue
			}
			h.notifHub.Broadcast(map[string]interface{}{
				"type": "release_tracker_execution",
				"notification": map[string]interface{}{
					"tracker_id":   trackerID,
					"tracker_name": tracker.Name,
					"tracker_type": tracker.TrackerType,
					"status":       status,
					"triggered_at": time.Now().UTC(),
				},
			})
		}
	}
}

func (h *ReleaseTrackerHandler) HandleCommandCompletion(commandID, status string) {
	h.NotifyComplete(commandID, status)
}

// ========== HTTP handlers ==========

func (h *ReleaseTrackerHandler) List(c *gin.Context) {
	trackers, err := h.db.ListReleaseTrackers()
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

	if req.TrackerType == "git" {
		// Git trackers always need a host+task to dispatch to.
		if req.HostID == "" || req.CustomTaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host_id and custom_task_id are required for git trackers"})
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
		// Docker trackers can run in monitor-only mode (no host/task).
		if req.HostID != "" && req.CustomTaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "custom_task_id is required when host_id is set"})
			return
		}
		if req.DockerImage == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "docker_image is required for docker trackers"})
			return
		}
		if req.DockerTag == "" {
			req.DockerTag = "latest"
		}
	}

	if req.NotifyChannels == nil {
		req.NotifyChannels = []string{}
	}

	created, err := h.db.CreateReleaseTracker(req)
	if err != nil {
		log.Printf("CreateReleaseTracker: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tracker"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"tracker": created})
}

func (h *ReleaseTrackerHandler) Get(c *gin.Context) {
	id := c.Param("id")
	t, err := h.db.GetReleaseTrackerByID(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}
	execs, _ := h.db.ListReleaseTrackerExecutions(id, 20)
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
	if req.TrackerType == "git" {
		if req.HostID == "" || req.CustomTaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "host_id and custom_task_id are required for git trackers"})
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
		if req.HostID != "" && req.CustomTaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "custom_task_id is required when host_id is set"})
			return
		}
		if req.DockerImage == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "docker_image is required for docker trackers"})
			return
		}
		if req.DockerTag == "" {
			req.DockerTag = "latest"
		}
	}

	if err := h.db.UpdateReleaseTracker(id, req); err != nil {
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
	if err := h.db.DeleteReleaseTracker(id); err != nil {
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

	t, err := h.db.GetReleaseTrackerByID(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}

	go h.checkOne(*t)
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

	t, err := h.db.GetReleaseTrackerByID(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get tracker"})
		return
	}

	if t.TrackerType == "docker" {
		if t.LatestImageDigest == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "aucune vérification initiale effectuée — attendez le prochain cycle de polling avant de déclencher manuellement"})
			return
		}
		tag := t.DockerTag
		if tag == "" {
			tag = "latest"
		}
		go h.dispatchDockerUpdate(*t, tag, t.LastReleaseTag, t.LatestImageDigest, t.LatestImageDigest)
	} else {
		if t.LastReleaseTag == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "aucune release initiale enregistrée — attendez le prochain cycle de polling avant de déclencher manuellement"})
			return
		}
		go h.dispatchGitRelease(*t, t.LastReleaseTag, "", "")
	}
	c.JSON(http.StatusOK, gin.H{"status": "execution scheduled"})
}

func (h *ReleaseTrackerHandler) GetExecutions(c *gin.Context) {
	id := c.Param("id")
	execs, err := h.db.ListReleaseTrackerExecutions(id, 50)
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

	t, err := h.db.GetReleaseTrackerByID(id)
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
		history, err = h.db.ListTrackerTagDigests(id, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load docker version history"})
			return
		}
	} else {
		providerClient := gitprovider.NewClient(t.Provider, h.cfg.GitHubToken)
		releases, ferr := providerClient.FetchReleaseHistory(t.RepoOwner, t.RepoName, limit)
		if ferr != nil {
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
