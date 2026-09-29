// Package releasetracker provides pure version-comparison helpers used by the
// release_trackers feature (handlers/release_trackers.go) and dashboard
// version-comparison snapshots. The legacy GitHub-polling Tracker was removed
// in favour of the configurable release_trackers table.
package releasetracker

import "strings"

// NormalizeDigest strips the "sha256:" prefix so two digests can be compared
// regardless of whether they were stored with or without it.
func NormalizeDigest(d string) string {
	return strings.TrimPrefix(d, "sha256:")
}

// IsVersionUpToDate decides whether a running container matches the latest
// known release for its tracker, in priority order:
//  1. Digest equality when both are known — strongest signal.
//  2. Tag equality (with channel-tag support: a running "v5.13.2" against a
//     latest "v5" is considered up-to-date).
//  3. Digest fallback for "latest" tags.
func IsVersionUpToDate(runningTag, runningDigest, latestTag, latestDigest string) bool {
	nd := NormalizeDigest(runningDigest)
	ld := NormalizeDigest(latestDigest)
	if nd != "" && ld != "" && nd == ld {
		return true
	}

	// When both tags are explicit (non-"latest") versions, tag equality wins.
	// Digest may legitimately differ across architectures or registry re-pushes.
	if runningTag != "latest" && latestTag != "latest" {
		r := NormalizeVersion(runningTag)
		l := NormalizeVersion(latestTag)
		if r == l {
			return true
		}
		// Support channel-like tags such as "v5" while running explicit patch versions like "v5.13.2".
		if l != "" && strings.HasPrefix(r, l+".") {
			return true
		}
		return false
	}

	// For "latest" tags, rely on digest comparison when available.
	if nd != "" && ld != "" {
		return nd == ld
	}
	return false
}

// NormalizeVersion strips a leading "v" so "v1.2.3" and "1.2.3" compare equal.
func NormalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

// ResolveContainerVersion picks the best human-readable version for a running
// container. A tag that names a version (v1.2, 14-alpine, 9) wins; a moving
// channel tag (latest, release, stable, ...) falls back to OCI/label hints,
// which can reveal the real release behind it.
func ResolveContainerVersion(imageTag string, labels map[string]string) string {
	if looksLikeVersionTag(imageTag) {
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

// looksLikeVersionTag reports whether a tag starts like a version number,
// optionally prefixed with "v".
func looksLikeVersionTag(tag string) bool {
	tag = strings.TrimPrefix(tag, "v")
	return tag != "" && tag[0] >= '0' && tag[0] <= '9'
}
