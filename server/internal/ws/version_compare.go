package ws

import (
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

func (h *WSHandler) buildVersionComparisons() ([]models.VersionComparison, error) {
	trackers, err := h.db.ListReleaseTrackers()
	if err != nil {
		return nil, err
	}

	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		return nil, err
	}

	digestTagMap, _ := h.db.GetAllTrackerTagDigests()
	if digestTagMap == nil {
		digestTagMap = make(map[string]string)
	}

	var comparisons []models.VersionComparison
	for _, tracker := range trackers {
		if tracker.DockerImage == "" || tracker.LastReleaseTag == "" {
			continue
		}

		releaseURL := ""
		if tracker.LastExecution != nil {
			releaseURL = tracker.LastExecution.ReleaseURL
		}

		matched := false
		for _, container := range containers {
			if container.HostID != tracker.HostID {
				continue
			}
			if container.Image != tracker.DockerImage && container.Image+":"+container.ImageTag != tracker.DockerImage {
				continue
			}

			nd := normalizeDigest(container.ImageDigest)
			ld := normalizeDigest(tracker.LatestImageDigest)
			isUpToDate := isVersionUpToDate(container.ImageTag, container.ImageDigest, tracker.LastReleaseTag, tracker.LatestImageDigest)
			updateConfirmed := !isUpToDate && nd != "" && ld != ""

			runningVersion := resolveContainerVersion(container.ImageTag, container.Labels)
			if runningVersion == "latest" && nd != "" {
				if nd == ld {
					runningVersion = tracker.LastReleaseTag
				} else if historicTag, ok := digestTagMap[tracker.ID+"|"+nd]; ok {
					runningVersion = historicTag
				}
			}
			if runningVersion == "latest" {
				runningVersion = ""
			}

			comparisons = append(comparisons, models.VersionComparison{
				DockerImage:     tracker.DockerImage,
				RunningVersion:  runningVersion,
				LatestVersion:   tracker.LastReleaseTag,
				IsUpToDate:      isUpToDate,
				UpdateConfirmed: updateConfirmed,
				RepoOwner:       tracker.RepoOwner,
				RepoName:        tracker.RepoName,
				ReleaseURL:      releaseURL,
				HostID:          tracker.HostID,
				Hostname:        tracker.HostName,
			})
			matched = true
		}

		if !matched {
			comparisons = append(comparisons, models.VersionComparison{
				DockerImage:   tracker.DockerImage,
				LatestVersion: tracker.LastReleaseTag,
				IsUpToDate:    false,
				RepoOwner:     tracker.RepoOwner,
				RepoName:      tracker.RepoName,
				ReleaseURL:    releaseURL,
				HostID:        tracker.HostID,
				Hostname:      tracker.HostName,
			})
		}
	}

	return comparisons, nil
}

func normalizeDigest(d string) string {
	return strings.TrimPrefix(d, "sha256:")
}

func isVersionUpToDate(runningTag, runningDigest, latestTag, latestDigest string) bool {
	// When both tags are explicit (non-"latest") versions, tag equality wins.
	// Digest may legitimately differ across architectures or registry re-pushes.
	if runningTag != "latest" && latestTag != "latest" {
		return normalizeVersion(runningTag) == normalizeVersion(latestTag)
	}

	// For "latest" tags, rely on digest comparison when available.
	nd := normalizeDigest(runningDigest)
	ld := normalizeDigest(latestDigest)
	if nd != "" && ld != "" {
		return nd == ld
	}
	return false
}

func normalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

func resolveContainerVersion(imageTag string, labels map[string]string) string {
	if imageTag != "latest" {
		return imageTag
	}
	for _, key := range []string{
		"org.opencontainers.image.version",
		"org.label-schema.version",
		"version",
	} {
		if v := labels[key]; v != "" {
			return v
		}
	}
	return imageTag
}
