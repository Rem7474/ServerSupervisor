package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
)

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
