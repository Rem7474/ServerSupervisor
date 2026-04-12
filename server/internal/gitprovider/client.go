package gitprovider

import (
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// Client is the unified interface for all Git providers
type Client interface {
	// FetchLatestRelease returns the latest release for the given repository.
	// If no release is found, it falls back to the latest tag.
	// Returns (tag, releaseURL, releaseName, error)
	FetchLatestRelease(owner, repo string) (string, string, string, error)

	// FetchReleaseHistory returns recent releases/tags for the given repository.
	// The list is ordered from newest to oldest.
	FetchReleaseHistory(owner, repo string, limit int) ([]Release, error)

	// FetchDockerManifestDigest returns the SHA256 digest of a Docker image manifest for tag.
	FetchDockerManifestDigest(imageName, tag string) (string, error)

	// FetchDockerVersionForDigest finds a versioned tag that matches the given manifest digest.
	// Returns "" if the version cannot be resolved.
	FetchDockerVersionForDigest(imageName, digest string) string
}

// Release contains metadata about a Git release
type Release struct {
	TagName     string
	Name        string
	PublishedAt time.Time
	HTMLURL     string
	Prerelease  bool
	Draft       bool
}

// NewClient creates a new Git provider client.
// Supported providers: "github", "gitlab", "gitea"
func NewClient(provider, authToken string) Client {
	switch provider {
	case "gitlab":
		return newGitLabClient(authToken)
	case "gitea":
		return newGiteaClient(authToken)
	default:
		return newGitHubClient(authToken)
	}
}

// Helper to convert models.GitHubRelease to Release
func toRelease(gh *models.GitHubRelease) *Release {
	if gh == nil {
		return nil
	}
	return &Release{
		TagName:     gh.TagName,
		Name:        gh.Name,
		PublishedAt: gh.PublishedAt,
		HTMLURL:     gh.HTMLURL,
		Prerelease:  gh.Prerelease,
		Draft:       gh.Draft,
	}
}
