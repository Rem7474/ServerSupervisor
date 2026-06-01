package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

func (h *ReleaseTrackerHandler) dispatchGitRelease(ctx context.Context, t models.ReleaseTracker, tag, releaseURL, releaseName string) {
	// Skip if already running
	running, _ := h.db.GetRunningExecutionForReleaseTracker(ctx, t.ID)
	if running {
		slog.InfoContext(ctx, fmt.Sprintf("Git tracker %s: skipping dispatch (already running)", t.Name))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Git tracker %s: failed to create execution: %v", t.Name, err))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Git tracker %s: failed to create command: %v", t.Name, err))
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
		slog.InfoContext(ctx, fmt.Sprintf("Docker tracker %s: skipping dispatch (already running)", t.Name))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Docker tracker %s: failed to create execution: %v", t.Name, err))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Docker tracker %s: failed to create command: %v", t.Name, err))
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
		slog.InfoContext(ctx, fmt.Sprintf("Compose tracker %s: skipping dispatch (already running)", t.Name))
		_ = h.db.UpdateReleaseTrackerLastSeen(ctx, t.ID, resolvedVersion, false)
		return
	}

	if newDigest != "" {
		deployed, _ := h.db.GetDeployedImageDigest(ctx, t.HostID, t.DockerImage, t.DockerTag)
		if deployed != "" && deployed == newDigest {
			slog.InfoContext(ctx, fmt.Sprintf("Compose tracker %s: deployed digest already current, skipping update", t.Name))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Compose tracker %s: failed to create execution: %v", t.Name, err))
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
		slog.ErrorContext(ctx, fmt.Sprintf("Compose tracker %s: failed to create command: %v", t.Name, err))
		now := time.Now()
		_ = h.db.UpdateReleaseTrackerExecutionStatus(ctx, exec.ID, "failed", &now)
		return
	}

	_ = h.db.UpdateReleaseTrackerExecutionCommandID(ctx, exec.ID, result.Command.ID)
	_ = h.db.MarkReleaseTrackerTriggered(ctx, t.ID)
}
