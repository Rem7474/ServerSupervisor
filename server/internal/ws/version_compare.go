package ws

import (
	"context"
	"sort"
	"sync"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/releasetracker"
	"github.com/serversupervisor/server/internal/safego"
)

// buildVersionComparisons produces the version-badge rows consumed by the
// dashboard / host detail / docker page snapshots.
//
// It emits a superset of what it used to: one row per release tracker exactly
// as before (tracker rows keep driving the "voir le suivi" / "déclencher"
// actions and aggregate every container sharing the tracker's image), plus one
// ambient row per running container group that no tracker covers, resolved from
// the docker_image_versions cache the internal/services/dockerversions engine
// refreshes. That's what makes the badge show for every container instead of
// only for the handful someone hand-configured a tracker for.
//
// The pure version logic still lives in the releasetracker package — this
// method owns only the WS-handler-side DB orchestration.
func (h *WSHandler) buildVersionComparisons(ctx context.Context) ([]models.VersionComparison, error) {
	// The four reads are independent — fetch them concurrently so the slowest
	// (GetAllDockerContainers) dominates instead of the sum.
	var (
		trackers      []models.ReleaseTracker
		trackersErr   error
		containers    []models.DockerContainer
		containersErr error
		digestTagMap  map[string]string
		imageVersions []models.DockerImageVersion
	)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildVersionComparisons.trackers")
		trackers, trackersErr = h.db.ListReleaseTrackers(ctx)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildVersionComparisons.containers")
		containers, containersErr = h.db.GetAllDockerContainers(ctx)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildVersionComparisons.digestTagMap")
		digestTagMap, _ = h.db.GetAllTrackerTagDigests(ctx)
	}()
	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildVersionComparisons.imageVersions")
		imageVersions, _ = h.db.ListDockerImageVersions(ctx)
	}()
	wg.Wait()

	if trackersErr != nil {
		return nil, trackersErr
	}
	if containersErr != nil {
		return nil, containersErr
	}
	if digestTagMap == nil {
		digestTagMap = make(map[string]string)
	}

	// Index containers by host so each tracker scans only its own host's
	// containers (was O(trackers × all containers)).
	containersByHost := make(map[string][]models.DockerContainer, len(containers))
	for _, c := range containers {
		containersByHost[c.HostID] = append(containersByHost[c.HostID], c)
	}

	// Container groups already explained by a tracker row, so the ambient pass
	// below doesn't emit a second, conflicting row for them.
	covered := make(map[string]struct{})

	comparisons := buildTrackerComparisons(trackers, containersByHost, digestTagMap, covered)
	comparisons = append(comparisons, buildAmbientComparisons(containers, imageVersions, covered)...)
	return comparisons, nil
}

// containerGroupKey identifies one (host, image, tag) group — the unit the
// frontend looks a container's badge up by.
func containerGroupKey(hostID, image, tag string) string {
	return hostID + "|" + image + "|" + normalizeTagForCompare(tag)
}

func normalizeTagForCompare(tag string) string {
	if tag == "" {
		return "latest"
	}
	return tag
}

// buildTrackerComparisons reproduces the pre-existing tracker-driven rows
// unchanged (including the no-matching-container row a tracker still gets), and
// records which container groups they cover.
func buildTrackerComparisons(
	trackers []models.ReleaseTracker,
	containersByHost map[string][]models.DockerContainer,
	digestTagMap map[string]string,
	covered map[string]struct{},
) []models.VersionComparison {
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

		for _, container := range containersByHost[tracker.HostID] {
			if container.Image != tracker.DockerImage && container.Image+":"+container.ImageTag != tracker.DockerImage {
				continue
			}
			covered[containerGroupKey(container.HostID, container.Image, container.ImageTag)] = struct{}{}

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
				Status:          comparisonStatus(aggIsUpToDate, aggRunningVersion, aggUpdateConfirmed),
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
				Status:        models.VersionStatusUnknown,
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
	return comparisons
}

// buildAmbientComparisons emits one row per (host, image, tag) group with no
// tracker, resolved from the docker_image_versions cache. A group whose image
// was never successfully checked (private registry with no matching credential,
// registry down, cache not warm yet) still gets a row — an explicit "version
// inconnue" with the reason attached beats no badge at all, which the user
// can't tell apart from "nothing to report".
func buildAmbientComparisons(
	containers []models.DockerContainer,
	imageVersions []models.DockerImageVersion,
	covered map[string]struct{},
) []models.VersionComparison {
	if len(containers) == 0 {
		return nil
	}
	cache := make(map[string]models.DockerImageVersion, len(imageVersions))
	for _, iv := range imageVersions {
		cache[iv.Image+"|"+normalizeTagForCompare(iv.ImageTag)] = iv
	}

	type group struct {
		container models.DockerContainer
		count     int
	}
	groups := make(map[string]*group)
	order := make([]string, 0)
	for _, c := range containers {
		if c.Image == "" {
			continue
		}
		key := containerGroupKey(c.HostID, c.Image, c.ImageTag)
		if _, skip := covered[key]; skip {
			continue
		}
		if g, ok := groups[key]; ok {
			g.count++
			// Prefer a container that actually knows its digest as the group's
			// representative — that's the one that can produce a confirmed verdict.
			if g.container.ImageDigest == "" && c.ImageDigest != "" {
				g.container = c
			}
			continue
		}
		groups[key] = &group{container: c, count: 1}
		order = append(order, key)
	}

	// Deterministic output: the WS layer hashes the snapshot to suppress
	// unchanged pushes, so map iteration order would cause phantom diffs.
	sort.Strings(order)

	out := make([]models.VersionComparison, 0, len(order))
	for _, key := range order {
		g := groups[key]
		out = append(out, ambientComparison(g.container, g.count, cache[g.container.Image+"|"+normalizeTagForCompare(g.container.ImageTag)]))
	}
	return out
}

// ambientComparison compares one container group against the cached registry
// state for its image.
func ambientComparison(c models.DockerContainer, count int, iv models.DockerImageVersion) models.VersionComparison {
	tag := normalizeTagForCompare(c.ImageTag)
	runningVersion := releasetracker.ResolveContainerVersion(c.ImageTag, c.Labels)
	if runningVersion == "latest" {
		runningVersion = ""
	}

	cmp := models.VersionComparison{
		DockerImage:    c.Image,
		ImageTag:       tag,
		RunningVersion: runningVersion,
		ContainerCount: count,
		HostID:         c.HostID,
		Hostname:       c.Hostname,
	}

	if iv.LatestDigest == "" {
		// Never successfully checked: report unknown with the recorded reason
		// rather than inventing an "up to date" or "outdated" verdict.
		cmp.Status = models.VersionStatusUnknown
		cmp.LastError = iv.LastError
		return cmp
	}

	latestVersion := iv.LatestTag
	if latestVersion == "" {
		latestVersion = tag
	}
	effectiveTag := tag
	if effectiveTag == "latest" && runningVersion != "" {
		effectiveTag = runningVersion
	}

	nd := releasetracker.NormalizeDigest(c.ImageDigest)
	ld := releasetracker.NormalizeDigest(iv.LatestDigest)
	isUpToDate := releasetracker.IsVersionUpToDate(effectiveTag, c.ImageDigest, latestVersion, iv.LatestDigest)
	// The deployed digest matching the registry's is the strongest possible
	// signal — surface the resolved release name for it too.
	if nd != "" && nd == ld && iv.LatestTag != "" {
		cmp.RunningVersion = iv.LatestTag
		runningVersion = iv.LatestTag
	}

	cmp.LatestVersion = latestVersion
	cmp.IsUpToDate = isUpToDate
	cmp.UpdateConfirmed = !isUpToDate && nd != "" && ld != ""
	cmp.Status = comparisonStatus(isUpToDate, runningVersion, cmp.UpdateConfirmed)
	// A container with no known digest and a moving tag can't be classified
	// either way from a tag comparison alone.
	if !isUpToDate && !cmp.UpdateConfirmed && nd == "" && tag == "latest" && runningVersion == "" {
		cmp.Status = models.VersionStatusUnknown
	}
	return cmp
}

// comparisonStatus centralises the up-to-date / update-available / unknown
// verdict that the two Docker tables used to each compute in their own template.
func comparisonStatus(isUpToDate bool, runningVersion string, updateConfirmed bool) string {
	switch {
	case isUpToDate:
		return models.VersionStatusUpToDate
	case runningVersion != "" || updateConfirmed:
		return models.VersionStatusUpdateAvailable
	default:
		return models.VersionStatusUnknown
	}
}
