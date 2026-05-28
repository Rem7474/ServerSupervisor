package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

// validateDockerTracker normalizes and validates a docker tracker's deployment
// mode. Returns (httpStatus, message); status 0 means valid. Compose mode
// requires host + project; custom mode keeps the legacy monitor-only/task rules.
func validateDockerTracker(req *models.ReleaseTracker) (int, string) {
	if req.UpdateAction == "" {
		req.UpdateAction = "custom"
	}
	if req.UpdateAction != "custom" && req.UpdateAction != "compose" {
		return http.StatusBadRequest, "update_action must be 'custom' or 'compose'"
	}
	if req.DockerImage == "" {
		return http.StatusBadRequest, "docker_image is required for docker trackers"
	}
	if req.DockerTag == "" {
		req.DockerTag = "latest"
	}
	if req.UpdateAction == "compose" {
		if req.HostID == "" || req.ComposeProject == "" {
			return http.StatusBadRequest, "host_id and compose_project are required for compose update mode"
		}
	} else if req.HostID != "" && req.CustomTaskID == "" {
		return http.StatusBadRequest, "custom_task_id is required when host_id is set"
	}
	if req.HealthcheckTimeoutSec < 0 || req.HealthcheckTimeoutSec > 3600 {
		return http.StatusBadRequest, "healthcheck_timeout_sec must be between 0 and 3600"
	}
	return 0, ""
}

type ReleaseTrackerHandler struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	notifHub   *ws.NotificationHub
	pollerCtx  context.Context // detached ctx for goroutines fired from handlers (check-now)
	cancel     context.CancelFunc
}

func NewReleaseTrackerHandler(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *ReleaseTrackerHandler {
	return &ReleaseTrackerHandler{
		db:         db,
		cfg:        cfg,
		dispatcher: dispatcher,
		notifHub:   notifHub,
		pollerCtx:  context.Background(), // placeholder; real ctx is set in StartPoller
	}
}

// StartPoller begins periodic polling of release trackers.
// The provided parent ctx is propagated to every DB call; cancelling it (or
// calling StopPoller) terminates the loop.
func (h *ReleaseTrackerHandler) StartPoller(parent context.Context) {
	interval := h.cfg.GitHubPollInterval
	if interval == 0 {
		interval = 15 * time.Minute
	}
	log.Printf("Release tracker poller started (interval: %v)", interval)

	ctx, cancel := context.WithCancel(parent)
	h.pollerCtx = ctx
	h.cancel = cancel

	go h.checkAll(ctx)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.checkAll(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (h *ReleaseTrackerHandler) StopPoller() {
	if h.cancel != nil {
		h.cancel()
	}
}

func (h *ReleaseTrackerHandler) checkAll(ctx context.Context) {
	trackers, err := h.db.GetEnabledReleaseTrackers(ctx)
	if err != nil {
		log.Printf("Release tracker poller: failed to fetch trackers: %v", err)
		return
	}
	for _, t := range trackers {
		if ctx.Err() != nil {
			return
		}
		h.checkOne(ctx, t)
	}
}

func (h *ReleaseTrackerHandler) checkOne(ctx context.Context, t models.ReleaseTracker) {
	switch t.TrackerType {
	case "docker":
		h.checkOneDocker(ctx, t)
	default: // "git"
		h.checkOneGit(ctx, t)
	}
}

// checkOneGit polls a git provider for new releases.
func (h *ReleaseTrackerHandler) checkOneGit(ctx context.Context, t models.ReleaseTracker) {
	providerClient := gitprovider.NewClient(t.Provider, h.cfg.GitHubToken)
	tag, releaseURL, releaseName, err := providerClient.FetchLatestRelease(t.RepoOwner, t.RepoName)
	cooldown := time.Duration(t.CooldownHours) * time.Hour
	if err != nil {
		log.Printf("Git tracker %s (%s/%s): fetch error: %v", t.Name, t.RepoOwner, t.RepoName, err)
		_ = h.db.UpdateReleaseTrackerError(ctx, t.ID, err.Error())
		return
	}

	if tag == "" {
		_ = h.db.UpdateReleaseTrackerError(ctx, t.ID, "aucune release ou tag trouvé sur ce dépôt")
		return
	}

	// First check — just record the current tag without triggering
	if t.LastReleaseTag == "" {
		log.Printf("Git tracker %s: initial tag recorded: %s", t.Name, tag)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, tag, false)
		return
	}

	// No change
	if tag == t.LastReleaseTag {
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, "", false)
		if cooldown > 0 && t.HostID != "" && t.CustomTaskID != "" && t.LastReleaseDetectedAt != nil && (t.LastTriggeredAt == nil || t.LastTriggeredAt.Before(*t.LastReleaseDetectedAt)) {
			if time.Since(*t.LastReleaseDetectedAt) >= cooldown {
				log.Printf("Git tracker %s: cooldown elapsed (%dh) for %s → dispatching", t.Name, t.CooldownHours, tag)
				h.dispatchGitRelease(ctx, t, tag, releaseURL, releaseName)
			}
		}
		return
	}

	// New release detected
	_ = h.db.UpdateReleaseTrackerDetected(ctx, t.ID, tag)
	h.notifyReleaseTrackerDetected(t, tag, releaseURL, releaseName)
	if t.HostID == "" || t.CustomTaskID == "" {
		log.Printf("Git tracker %s: new release %s detected (monitor-only, no task dispatched)", t.Name, tag)
		return
	}
	if cooldown > 0 {
		log.Printf("Git tracker %s: new release %s detected (cooldown=%dh) — deployment delayed", t.Name, tag, t.CooldownHours)
		return
	}

	log.Printf("Git tracker %s: new release %s (was %s) → dispatching task %s on host %s",
		t.Name, tag, t.LastReleaseTag, t.CustomTaskID, t.HostID)

	h.dispatchGitRelease(ctx, t, tag, releaseURL, releaseName)
}

// checkOneDocker polls the Docker registry for a new image digest.
func (h *ReleaseTrackerHandler) checkOneDocker(ctx context.Context, t models.ReleaseTracker) {
	if t.DockerImage == "" {
		_ = h.db.UpdateReleaseTrackerError(ctx, t.ID, "docker_image manquant")
		return
	}
	tag := t.DockerTag
	if tag == "" {
		tag = "latest"
	}

	cooldown := time.Duration(t.CooldownHours) * time.Hour

	// Private-registry credentials (optional) authenticate manifest polling.
	var regUser, regPass string
	if t.RegistryCredentialsID != "" {
		regUser, regPass, _ = h.db.GetRegistryCredentialAuth(ctx, t.RegistryCredentialsID)
	}
	digest, err := gitprovider.FetchDockerManifestDigestWithAuth(t.DockerImage, tag, h.cfg.GitHubToken, regUser, regPass)
	if err != nil {
		log.Printf("Docker tracker %s (%s:%s): fetch error: %v", t.Name, t.DockerImage, tag, err)
		errMsg := err.Error()
		if tag == "latest" && strings.Contains(strings.ToLower(errMsg), "status 404") {
			errMsg = "tag latest introuvable pour cette image; utilisez un tag versionne (ex: v4, v4.4, v4.4.1)"
		}
		_ = h.db.UpdateReleaseTrackerError(ctx, t.ID, errMsg)
		return
	}
	if digest == "" {
		_ = h.db.UpdateReleaseTrackerError(ctx, t.ID, "digest vide retourné par le registre")
		return
	}

	// For mutable tags ("latest", major/minor channels like "v4"/"v4.4"),
	// resolve the exact version from the digest when possible.
	// Docker Hub: uses hub.docker.com API (tags + digest in one call).
	// Others: enumerates semver-looking tags and HEADs each manifest.
	resolvedVersion := tag
	if shouldResolveDockerTag(tag) {
		if v := gitprovider.FetchDockerVersionForDigestWithAuth(t.DockerImage, digest, h.cfg.GitHubToken, regUser, regPass); v != "" {
			resolvedVersion = v
			if v != tag {
				log.Printf("Docker tracker %s: resolved mutable tag %q to %q", t.Name, tag, v)
			}
		}
	}

	// First check — record current digest without triggering
	if t.LatestImageDigest == "" {
		log.Printf("Docker tracker %s: initial digest recorded for %s:%s", t.Name, t.DockerImage, tag)
		_ = h.db.UpdateReleaseTrackerDigest(ctx, t.ID, digest)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		_ = h.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
		return
	}

	// No change
	if digest == t.LatestImageDigest {
		// If we previously stored "latest" as the tag but can now resolve a version, update it.
		if resolvedVersion != tag && t.LastReleaseTag == tag {
			_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
			_ = h.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
		} else {
			_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, "", false)
		}

		if cooldown > 0 && trackerHasDispatchTarget(t) && t.LastReleaseDetectedAt != nil && (t.LastTriggeredAt == nil || t.LastTriggeredAt.Before(*t.LastReleaseDetectedAt)) {
			if time.Since(*t.LastReleaseDetectedAt) >= cooldown {
				log.Printf("Docker tracker %s: cooldown elapsed (%dh) for %s:%s → dispatching", t.Name, t.CooldownHours, t.DockerImage, tag)
				h.dispatchDockerTracker(ctx, t, tag, resolvedVersion, digest, digest)
			}
		}
		return
	}

	// New image detected
	oldDigest := t.LatestImageDigest
	_ = h.db.UpdateReleaseTrackerDigest(ctx, t.ID, digest)
	_ = h.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
	_ = h.db.UpdateReleaseTrackerDetected(ctx, t.ID, resolvedVersion)
	h.notifyReleaseTrackerDetected(t, resolvedVersion, "", t.DockerImage+":"+tag)

	// Monitor-only mode: no dispatch target configured, just record the version.
	if !trackerHasDispatchTarget(t) {
		log.Printf("Docker tracker %s: new digest for %s:%s (monitor-only, no task dispatched)", t.Name, t.DockerImage, tag)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}

	if cooldown > 0 {
		log.Printf("Docker tracker %s: new digest for %s:%s detected (cooldown=%dh) — deployment delayed", t.Name, t.DockerImage, tag, t.CooldownHours)
		return
	}

	log.Printf("Docker tracker %s: new digest for %s:%s → dispatching (%s) on host %s",
		t.Name, t.DockerImage, tag, t.UpdateAction, t.HostID)
	h.dispatchDockerTracker(ctx, t, tag, resolvedVersion, oldDigest, digest)
}

func shouldResolveDockerTag(tag string) bool {
	t := strings.TrimSpace(strings.ToLower(tag))
	if t == "" || t == "latest" {
		return true
	}

	t = strings.TrimPrefix(t, "v")
	parts := strings.Split(t, ".")
	if len(parts) != 1 && len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}

func (h *ReleaseTrackerHandler) dispatchGitRelease(ctx context.Context, t models.ReleaseTracker, tag, releaseURL, releaseName string) {
	// Skip if already running
	running, _ := h.db.GetRunningExecutionForReleaseTracker(ctx, t.ID)
	if running {
		log.Printf("Git tracker %s: skipping dispatch (already running)", t.Name)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, tag, false)
		return
	}

	exec, err := h.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
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
	result, err := h.dispatcher.Create(ctx, dispatch.Request{
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
		_ = h.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = h.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

func (h *ReleaseTrackerHandler) dispatchDockerUpdate(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, oldDigest, newDigest string) {
	running, _ := h.db.GetRunningExecutionForReleaseTracker(ctx, t.ID)
	if running {
		log.Printf("Docker tracker %s: skipping dispatch (already running)", t.Name)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}

	imageFull := t.DockerImage + ":" + tag
	exec, err := h.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
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
	result, err := h.dispatcher.Create(ctx, dispatch.Request{
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
		_ = h.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = h.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

// trackerHasDispatchTarget reports whether a tracker is configured to deploy
// (vs monitor-only). Compose mode needs host + project; custom needs host + task.
func trackerHasDispatchTarget(t models.ReleaseTracker) bool {
	if t.UpdateAction == "compose" {
		return t.HostID != "" && t.ComposeProject != ""
	}
	return t.HostID != "" && t.CustomTaskID != ""
}

// dispatchDockerTracker routes a docker tracker to the native compose module or
// the legacy tasks.yaml command depending on its configured update_action.
func (h *ReleaseTrackerHandler) dispatchDockerTracker(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, oldDigest, newDigest string) {
	if t.UpdateAction == "compose" {
		h.dispatchComposeUpdate(ctx, t, tag, resolvedVersion, newDigest)
		return
	}
	h.dispatchDockerUpdate(ctx, t, tag, resolvedVersion, oldDigest, newDigest)
}

// dispatchComposeUpdate dispatches the native compose update module (pull + up
// -d, with optional hooks/healthcheck/rollback/cleanup). It skips the dispatch
// when the digest already deployed on the host matches the registry's latest
// (drift detection — the stack is already current).
func (h *ReleaseTrackerHandler) dispatchComposeUpdate(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, newDigest string) {
	running, _ := h.db.GetRunningExecutionForReleaseTracker(ctx, t.ID)
	if running {
		log.Printf("Compose tracker %s: skipping dispatch (already running)", t.Name)
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}

	if newDigest != "" {
		deployed, _ := h.db.GetDeployedImageDigest(ctx, t.HostID, t.DockerImage, t.DockerTag)
		if deployed != "" && deployed == newDigest {
			log.Printf("Compose tracker %s: deployed digest already current, skipping update", t.Name)
			_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
			return
		}
	}

	imageFull := t.DockerImage + ":" + tag
	exec, err := h.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID:   t.ID,
		TagName:     resolvedVersion,
		ReleaseName: imageFull,
		Status:      "pending",
	})
	if err != nil {
		log.Printf("Compose tracker %s: failed to create execution: %v", t.Name, err)
		return
	}

	payload := map[string]interface{}{
		"service":                 t.ComposeService,
		"pre_task_id":             t.PreUpdateTaskID,
		"post_task_id":            t.PostUpdateTaskID,
		"cleanup":                 t.CleanupAfterUpdate,
		"healthcheck_timeout_sec": t.HealthcheckTimeoutSec,
		"rollback":                t.RollbackOnFailure,
		"env": map[string]string{
			"SS_IMAGE_NAME":    imageFull,
			"SS_IMAGE_TAG":     tag,
			"SS_IMAGE_VERSION": resolvedVersion,
			"SS_NEW_DIGEST":    newDigest,
			"SS_TRACKER_NAME":  t.Name,
		},
	}
	envPayload, _ := json.Marshal(payload)

	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := h.dispatcher.Create(ctx, dispatch.Request{
		HostID:      t.HostID,
		Module:      "compose",
		Action:      "update",
		Target:      t.ComposeProject,
		Payload:     string(envPayload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "compose_tracker_trigger",
			HostID:    t.HostID,
			IPAddress: "poller",
			Details:   fmt.Sprintf(`{"tracker_id":%q,"project":%q,"image":%q,"digest":%q}`, t.ID, t.ComposeProject, imageFull, newDigest),
		},
	})
	if err != nil {
		log.Printf("Compose tracker %s: failed to create command: %v", t.Name, err)
		now := time.Now()
		_ = h.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = h.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

func (h *ReleaseTrackerHandler) notifyReleaseTrackerDetected(t models.ReleaseTracker, version, releaseURL, releaseName string) {
	if h.notifHub == nil || len(t.NotifyChannels) == 0 {
		return
	}

	hasBrowser := false
	for _, ch := range t.NotifyChannels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	label := "Git"
	if t.TrackerType == "docker" {
		label = "Docker"
	}

	h.notifHub.Broadcast(map[string]interface{}{
		"type": "release_tracker_detected",
		"notification": map[string]interface{}{
			"tracker_id":   t.ID,
			"tracker_name": t.Name,
			"tracker_type": t.TrackerType,
			"version":      version,
			"release_url":  releaseURL,
			"release_name": releaseName,
			"status":       "detected",
			"label":        label,
			"triggered_at": time.Now().UTC(),
		},
	})
}

// NotifyComplete is called by the agent handler when a command completes.
// It is fire-and-forget from an HTTP goroutine, so it uses h.pollerCtx
// (cancelled at shutdown) rather than the request ctx (which is already done).
func (h *ReleaseTrackerHandler) NotifyComplete(commandID, status string) {
	ctx := h.pollerCtx
	trackerID, notifyOnRelease, channels, err := h.db.UpdateReleaseTrackerExecutionByCommandID(ctx, commandID, status)
	if err != nil {
		return // not a tracker command
	}

	if !notifyOnRelease || len(channels) == 0 {
		return
	}

	tracker, err := h.db.GetReleaseTrackerByID(ctx, trackerID)
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
					"tracker_id":   tracker.ID,
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
		log.Printf("CreateReleaseTracker: %v", err)
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
		log.Printf("ListTrackableContainers: %v", err)
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
			log.Printf("CreateBulk: failed to create %q: %v", t.Name, err)
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
		log.Printf("Release tracker history: tracker not found (id=%s)", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "tracker not found"})
		return
	}
	if err != nil {
		log.Printf("Release tracker history: failed to load tracker (id=%s): %v", id, err)
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
			log.Printf("Release tracker history: docker history load error (tracker=%s id=%s image=%s tag=%s limit=%d): %v", t.Name, t.ID, t.DockerImage, t.DockerTag, limit, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load docker version history"})
			return
		}
	} else {
		providerClient := gitprovider.NewClient(t.Provider, h.cfg.GitHubToken)
		releases, ferr := providerClient.FetchReleaseHistory(t.RepoOwner, t.RepoName, limit)
		if ferr != nil {
			log.Printf("Release tracker history: provider call failed (tracker=%s id=%s provider=%s repo=%s/%s limit=%d): %v", t.Name, t.ID, t.Provider, t.RepoOwner, t.RepoName, limit, ferr)
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
