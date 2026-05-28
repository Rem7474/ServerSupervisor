package ws

import (
	"context"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/releasetracker"
)

// buildVersionComparisons aggregates running containers per release tracker
// and produces a comparison row used by dashboard / host detail snapshots.
// The pure version logic lives in the releasetracker package — this method
// owns only the WS-handler-side DB orchestration.
func (h *WSHandler) buildVersionComparisons(ctx context.Context) ([]models.VersionComparison, error) {
	trackers, err := h.db.ListReleaseTrackers(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := h.db.GetAllDockerContainers(ctx)
	if err != nil {
		return nil, err
	}

	digestTagMap, _ := h.db.GetAllTrackerTagDigests(ctx)
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

		// Aggregate all containers that share this tracker's image into one entry.
		// Worst-case: outdated if any container is outdated; confirmed if any has both digests.
		matchCount := 0
		aggRunningVersion := ""
		aggIsUpToDate := true
		aggUpdateConfirmed := false

		for _, container := range containers {
			if container.HostID != tracker.HostID {
				continue
			}
			if container.Image != tracker.DockerImage && container.Image+":"+container.ImageTag != tracker.DockerImage {
				continue
			}

			nd := releasetracker.NormalizeDigest(container.ImageDigest)
			ld := releasetracker.NormalizeDigest(tracker.LatestImageDigest)

			// Resolve display version with digest priority: digest can reveal an exact
			// deployed release (e.g. v5.13.2) even if runtime tag stays broad (e.g. v5).
			runningVersion := releasetracker.ResolveContainerVersion(container.ImageTag, container.Labels)
			if nd != "" {
				if nd == ld {
					runningVersion = tracker.LastReleaseTag
				} else if historicTag, ok := digestTagMap[tracker.ID+"|"+nd]; ok && historicTag != "" {
					runningVersion = historicTag
				}
			}
			if runningVersion == "latest" {
				runningVersion = ""
			}

			// Use the resolved version as effective tag so that containers running
			// "latest" with an OCI label matching the release tag are considered up to date.
			effectiveTag := container.ImageTag
			if effectiveTag == "latest" && runningVersion != "" {
				effectiveTag = runningVersion
			}
			isUpToDate := releasetracker.IsVersionUpToDate(effectiveTag, container.ImageDigest, tracker.LastReleaseTag, tracker.LatestImageDigest)
			updateConfirmed := !isUpToDate && nd != "" && ld != ""

			matchCount++
			// Prefer a non-empty resolved version over empty.
			if aggRunningVersion == "" && runningVersion != "" {
				aggRunningVersion = runningVersion
			}
			// Worst-case: any outdated container makes the tracker outdated.
			if !isUpToDate {
				aggIsUpToDate = false
			}
			// Confirmed if any container has both digests available.
			if updateConfirmed {
				aggUpdateConfirmed = true
			}
		}

		if matchCount > 0 {
			comparisons = append(comparisons, models.VersionComparison{
				TrackerID:       tracker.ID,
				DockerImage:     tracker.DockerImage,
				RunningVersion:  aggRunningVersion,
				LatestVersion:   tracker.LastReleaseTag,
				IsUpToDate:      aggIsUpToDate,
				UpdateConfirmed: aggUpdateConfirmed,
				ContainerCount:  matchCount,
				CustomTaskID:    tracker.CustomTaskID,
				RepoOwner:       tracker.RepoOwner,
				RepoName:        tracker.RepoName,
				ReleaseURL:      releaseURL,
				HostID:          tracker.HostID,
				Hostname:        tracker.HostName,
			})
		} else {
			comparisons = append(comparisons, models.VersionComparison{
				TrackerID:     tracker.ID,
				DockerImage:   tracker.DockerImage,
				LatestVersion: tracker.LastReleaseTag,
				IsUpToDate:    false,
				CustomTaskID:  tracker.CustomTaskID,
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
