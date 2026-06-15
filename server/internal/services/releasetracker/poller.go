// Package releasetracker is the application/service layer for release trackers
// (git + docker) and registry credentials. The HTTP use-cases sit behind a
// Repository port; the polling/dispatch/notify pipeline is background work and
// uses the concrete *database.DB + gitprovider + dispatcher + notification hub.
package releasetracker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/ws"
)

// Poller runs the background release-tracking pipeline (poll → detect → dispatch →
// notify). Background work, so it uses the concrete *database.DB.
type Poller struct {
	db         *database.DB
	cfg        *config.Config
	dispatcher *dispatch.Dispatcher
	notifHub   *ws.NotificationHub
}

func NewPoller(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *Poller {
	return &Poller{db: db, cfg: cfg, dispatcher: dispatcher, notifHub: notifHub}
}

// CheckAll polls every enabled tracker once.
func (s *Poller) CheckAll(ctx context.Context) {
	trackers, err := s.db.GetEnabledReleaseTrackers(ctx)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Release tracker poller: failed to fetch trackers: %v", err))
		return
	}
	for _, t := range trackers {
		if ctx.Err() != nil {
			return
		}
		s.CheckOne(ctx, t)
	}
}

// CheckOne polls a single tracker (git or docker).
func (s *Poller) CheckOne(ctx context.Context, t models.ReleaseTracker) {
	switch t.TrackerType {
	case "docker":
		s.checkOneDocker(ctx, t)
	default: // "git"
		s.checkOneGit(ctx, t)
	}
}

func (s *Poller) checkOneGit(ctx context.Context, t models.ReleaseTracker) {
	providerClient := gitprovider.NewClient(t.Provider, s.cfg.GitHubToken)
	tag, releaseURL, releaseName, err := providerClient.FetchLatestRelease(t.RepoOwner, t.RepoName)
	cooldown := time.Duration(t.CooldownHours) * time.Hour
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Git tracker %s (%s/%s): fetch error: %v", t.Name, t.RepoOwner, t.RepoName, err))
		_ = s.db.UpdateReleaseTrackerError(ctx, t.ID, err.Error())
		return
	}
	if tag == "" {
		_ = s.db.UpdateReleaseTrackerError(ctx, t.ID, "aucune release ou tag trouvé sur ce dépôt")
		return
	}

	if t.LastReleaseTag == "" {
		slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: initial tag recorded: %s", t.Name, tag))
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, tag, false)
		return
	}

	if tag == t.LastReleaseTag {
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, "", false)
		if cooldown > 0 && t.HostID != "" && t.CustomTaskID != "" && t.LastReleaseDetectedAt != nil && (t.LastTriggeredAt == nil || t.LastTriggeredAt.Before(*t.LastReleaseDetectedAt)) {
			if time.Since(*t.LastReleaseDetectedAt) >= cooldown {
				slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: cooldown elapsed (%dh) for %s → dispatching", t.Name, t.CooldownHours, tag))
				s.DispatchGitRelease(ctx, t, tag, releaseURL, releaseName)
			}
		}
		return
	}

	_ = s.db.UpdateReleaseTrackerDetected(ctx, t.ID, tag)
	s.notifyDetected(t, tag, releaseURL, releaseName)
	if t.HostID == "" || t.CustomTaskID == "" {
		slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: new release %s detected (monitor-only, no task dispatched)", t.Name, tag))
		return
	}
	if cooldown > 0 {
		slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: new release %s detected (cooldown=%dh) — deployment delayed", t.Name, tag, t.CooldownHours))
		return
	}
	slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: new release %s (was %s) → dispatching task %s on host %s", t.Name, tag, t.LastReleaseTag, t.CustomTaskID, t.HostID))
	s.DispatchGitRelease(ctx, t, tag, releaseURL, releaseName)
}

func (s *Poller) checkOneDocker(ctx context.Context, t models.ReleaseTracker) {
	if t.DockerImage == "" {
		_ = s.db.UpdateReleaseTrackerError(ctx, t.ID, "docker_image manquant")
		return
	}
	tag := t.DockerTag
	if tag == "" {
		tag = "latest"
	}
	cooldown := time.Duration(t.CooldownHours) * time.Hour

	var regUser, regPass string
	if t.RegistryCredentialsID != "" {
		regUser, regPass, _ = s.db.GetRegistryCredentialAuth(ctx, t.RegistryCredentialsID)
	}
	digest, err := gitprovider.FetchDockerManifestDigestWithAuth(t.DockerImage, tag, s.cfg.GitHubToken, regUser, regPass)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Docker tracker %s (%s:%s): fetch error: %v", t.Name, t.DockerImage, tag, err))
		errMsg := err.Error()
		if tag == "latest" && strings.Contains(strings.ToLower(errMsg), "status 404") {
			errMsg = "tag latest introuvable pour cette image; utilisez un tag versionne (ex: v4, v4.4, v4.4.1)"
		}
		_ = s.db.UpdateReleaseTrackerError(ctx, t.ID, errMsg)
		return
	}
	if digest == "" {
		_ = s.db.UpdateReleaseTrackerError(ctx, t.ID, "digest vide retourné par le registre")
		return
	}

	resolvedVersion := tag
	if shouldResolveDockerTag(tag) {
		if v := gitprovider.FetchDockerVersionForDigestWithAuth(t.DockerImage, digest, s.cfg.GitHubToken, regUser, regPass); v != "" {
			resolvedVersion = v
			if v != tag {
				slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: resolved mutable tag %q to %q", t.Name, tag, v))
			}
		}
	}

	if t.LatestImageDigest == "" {
		slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: initial digest recorded for %s:%s", t.Name, t.DockerImage, tag))
		_ = s.db.UpdateReleaseTrackerDigest(ctx, t.ID, digest)
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		_ = s.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
		return
	}

	if digest == t.LatestImageDigest {
		if resolvedVersion != tag && t.LastReleaseTag == tag {
			_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
			_ = s.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
		} else {
			_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, "", false)
		}
		if cooldown > 0 && trackerHasDispatchTarget(t) && t.LastReleaseDetectedAt != nil && (t.LastTriggeredAt == nil || t.LastTriggeredAt.Before(*t.LastReleaseDetectedAt)) {
			if time.Since(*t.LastReleaseDetectedAt) >= cooldown {
				slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: cooldown elapsed (%dh) for %s:%s → dispatching", t.Name, t.CooldownHours, t.DockerImage, tag))
				s.DispatchDockerTracker(ctx, t, tag, resolvedVersion, digest, digest)
			}
		}
		return
	}

	oldDigest := t.LatestImageDigest
	_ = s.db.UpdateReleaseTrackerDigest(ctx, t.ID, digest)
	_ = s.db.StoreTrackerTagDigest(ctx, t.ID, resolvedVersion, digest)
	_ = s.db.UpdateReleaseTrackerDetected(ctx, t.ID, resolvedVersion)
	s.notifyDetected(t, resolvedVersion, "", t.DockerImage+":"+tag)

	if !trackerHasDispatchTarget(t) {
		slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: new digest for %s:%s (monitor-only, no task dispatched)", t.Name, t.DockerImage, tag))
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}
	if cooldown > 0 {
		slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: new digest for %s:%s detected (cooldown=%dh) — deployment delayed", t.Name, t.DockerImage, tag, t.CooldownHours))
		return
	}
	slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: new digest for %s:%s → dispatching (%s) on host %s", t.Name, t.DockerImage, tag, t.UpdateAction, t.HostID))
	s.DispatchDockerTracker(ctx, t, tag, resolvedVersion, oldDigest, digest)
}

// ===== dispatch =====

// DispatchGitRelease dispatches a git tracker's custom task with release env vars.
func (s *Poller) DispatchGitRelease(ctx context.Context, t models.ReleaseTracker, tag, releaseURL, releaseName string) {
	if running, _ := s.db.GetRunningExecutionForReleaseTracker(ctx, t.ID); running {
		slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: skipping dispatch (already running)", t.Name))
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, tag, false)
		return
	}
	exec, err := s.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID: t.ID, TagName: tag, ReleaseURL: releaseURL, ReleaseName: releaseName, Status: "pending",
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Git tracker %s: failed to create execution: %v", t.Name, err))
		return
	}
	envPayload, _ := json.Marshal(map[string]any{"env": map[string]string{
		"SS_REPO_NAME": t.RepoOwner + "/" + t.RepoName, "SS_TAG_NAME": tag,
		"SS_RELEASE_URL": releaseURL, "SS_RELEASE_NAME": releaseName, "SS_TRACKER_NAME": t.Name,
	}})
	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID: t.HostID, Module: "custom", Action: "run", Target: t.CustomTaskID,
		Payload: string(envPayload), TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username: username, Action: "release_trigger", HostID: t.HostID, IPAddress: "poller",
			Details: fmt.Sprintf(`{"tracker_id":%q,"repo":%q,"tag":%q}`, t.ID, t.RepoOwner+"/"+t.RepoName, tag),
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Git tracker %s: failed to create command: %v", t.Name, err))
		now := time.Now()
		_ = s.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}
	_ = s.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = s.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

// DispatchDockerTracker routes a docker tracker to the compose module or the
// legacy tasks.yaml command depending on its update_action.
func (s *Poller) DispatchDockerTracker(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, oldDigest, newDigest string) {
	if t.UpdateAction == "compose" {
		s.dispatchComposeUpdate(ctx, t, tag, resolvedVersion, newDigest)
		return
	}
	s.dispatchDockerUpdate(ctx, t, tag, resolvedVersion, oldDigest, newDigest)
}

func (s *Poller) dispatchDockerUpdate(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, oldDigest, newDigest string) {
	if running, _ := s.db.GetRunningExecutionForReleaseTracker(ctx, t.ID); running {
		slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: skipping dispatch (already running)", t.Name))
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}
	imageFull := t.DockerImage + ":" + tag
	exec, err := s.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID: t.ID, TagName: resolvedVersion, ReleaseName: imageFull, Status: "pending",
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Docker tracker %s: failed to create execution: %v", t.Name, err))
		return
	}
	envPayload, _ := json.Marshal(map[string]any{"env": map[string]string{
		"SS_IMAGE_NAME": imageFull, "SS_IMAGE_TAG": tag, "SS_IMAGE_VERSION": resolvedVersion,
		"SS_OLD_DIGEST": oldDigest, "SS_NEW_DIGEST": newDigest, "SS_TRACKER_NAME": t.Name,
	}})
	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID: t.HostID, Module: "custom", Action: "run", Target: t.CustomTaskID,
		Payload: string(envPayload), TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username: username, Action: "docker_tracker_trigger", HostID: t.HostID, IPAddress: "poller",
			Details: fmt.Sprintf(`{"tracker_id":%q,"image":%q,"digest":%q}`, t.ID, imageFull, newDigest),
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Docker tracker %s: failed to create command: %v", t.Name, err))
		now := time.Now()
		_ = s.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}
	_ = s.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = s.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

func (s *Poller) dispatchComposeUpdate(ctx context.Context, t models.ReleaseTracker, tag, resolvedVersion, newDigest string) {
	if running, _ := s.db.GetRunningExecutionForReleaseTracker(ctx, t.ID); running {
		slog.InfoContext(ctx, fmt.Sprintf("Compose tracker %s: skipping dispatch (already running)", t.Name))
		_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}
	if newDigest != "" {
		if deployed, _ := s.db.GetDeployedImageDigest(ctx, t.HostID, t.DockerImage, t.DockerTag); deployed != "" && deployed == newDigest {
			slog.InfoContext(ctx, fmt.Sprintf("Compose tracker %s: deployed digest already current, skipping update", t.Name))
			_ = s.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
			return
		}
	}
	imageFull := t.DockerImage + ":" + tag
	exec, err := s.db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID: t.ID, TagName: resolvedVersion, ReleaseName: imageFull, Status: "pending",
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Compose tracker %s: failed to create execution: %v", t.Name, err))
		return
	}
	payload := map[string]any{
		"service": t.ComposeService, "pre_task_id": t.PreUpdateTaskID, "post_task_id": t.PostUpdateTaskID,
		"cleanup": t.CleanupAfterUpdate, "healthcheck_timeout_sec": t.HealthcheckTimeoutSec, "rollback": t.RollbackOnFailure,
		"env": map[string]string{
			"SS_IMAGE_NAME": imageFull, "SS_IMAGE_TAG": tag, "SS_IMAGE_VERSION": resolvedVersion,
			"SS_NEW_DIGEST": newDigest, "SS_TRACKER_NAME": t.Name,
		},
	}
	envPayload, _ := json.Marshal(payload)
	username := fmt.Sprintf("tracker:%s", t.Name)
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID: t.HostID, Module: "compose", Action: "update", Target: t.ComposeProject,
		Payload: string(envPayload), TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username: username, Action: "compose_tracker_trigger", HostID: t.HostID, IPAddress: "poller",
			Details: fmt.Sprintf(`{"tracker_id":%q,"project":%q,"image":%q,"digest":%q}`, t.ID, t.ComposeProject, imageFull, newDigest),
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Compose tracker %s: failed to create command: %v", t.Name, err))
		now := time.Now()
		_ = s.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}
	_ = s.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = s.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}

// notifyDetected pushes a "release detected" browser notification.
func (s *Poller) notifyDetected(t models.ReleaseTracker, version, releaseURL, releaseName string) {
	if s.notifHub == nil || len(t.NotifyChannels) == 0 {
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
	s.notifHub.Broadcast(models.WSReleaseTrackerMessage{
		Type: "release_tracker_detected",
		Notification: models.WSReleaseTrackerNotification{
			TrackerID: t.ID, TrackerName: t.Name, TrackerType: t.TrackerType,
			Version: version, ReleaseURL: releaseURL, ReleaseName: releaseName,
			Status: "detected", Label: label, TriggeredAt: time.Now().UTC(),
		},
	})
}

// ===== pure helpers =====

// trackerHasDispatchTarget reports whether a tracker is configured to deploy.
func trackerHasDispatchTarget(t models.ReleaseTracker) bool {
	if t.UpdateAction == "compose" {
		return t.HostID != "" && t.ComposeProject != ""
	}
	return t.HostID != "" && t.CustomTaskID != ""
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
