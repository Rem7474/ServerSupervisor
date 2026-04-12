package gitprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

type gitHubClient struct {
	authToken string
	client    *http.Client
}

func newGitHubClient(authToken string) *gitHubClient {
	return &gitHubClient{
		authToken: authToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchLatestRelease returns the latest GitHub release with fallback to tags
func (c *gitHubClient) FetchLatestRelease(owner, repo string) (string, string, string, error) {
	release, err := c.fetchGitHubRelease(owner, repo)
	if err != nil {
		return "", "", "", err
	}
	if release.TagName != "" {
		return release.TagName, release.HTMLURL, release.Name, nil
	}

	// Fallback to tags
	tag, err := c.fetchGitHubTag(owner, repo)
	if err != nil {
		return "", "", "", err
	}
	return tag.TagName, tag.HTMLURL, tag.Name, nil
}

// FetchReleaseHistory returns recent GitHub releases with fallback to tags.
func (c *gitHubClient) FetchReleaseHistory(owner, repo string, limit int) ([]Release, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d", owner, repo, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return c.fetchGitHubTagHistory(owner, repo, limit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, githubAPIError(resp.StatusCode)
	}

	var releases []models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return c.fetchGitHubTagHistory(owner, repo, limit)
	}

	out := make([]Release, 0, len(releases))
	for _, r := range releases {
		publishedAt := r.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = time.Now().UTC()
		}
		out = append(out, Release{
			TagName:     r.TagName,
			Name:        r.Name,
			PublishedAt: publishedAt,
			HTMLURL:     r.HTMLURL,
			Prerelease:  r.Prerelease,
			Draft:       r.Draft,
		})
	}
	return out, nil
}

// FetchDockerManifestDigest returns the SHA256 digest from Docker Hub/registries
func (c *gitHubClient) FetchDockerManifestDigest(imageName, tag string) (string, error) {
	return fetchDockerManifestDigest(c.client, imageName, tag)
}

// FetchDockerVersionForDigest finds a versioned tag matching the given digest.
func (c *gitHubClient) FetchDockerVersionForDigest(imageName, digest string) string {
	return fetchDockerVersionForDigest(c.client, imageName, digest)
}

func (c *gitHubClient) fetchGitHubRelease(owner, repo string) (*models.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		// Try tags instead
		return &models.GitHubRelease{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, githubAPIError(resp.StatusCode)
	}

	var release models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (c *gitHubClient) fetchGitHubTag(owner, repo string) (*models.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, githubAPIError(resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return &models.GitHubRelease{}, nil
	}

	return &models.GitHubRelease{
		TagName: tags[0].Name,
		HTMLURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, tags[0].Name),
	}, nil
}

func (c *gitHubClient) fetchGitHubTagHistory(owner, repo string, limit int) ([]Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=%d", owner, repo, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, githubAPIError(resp.StatusCode)
	}

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(tags))
	for _, t := range tags {
		out = append(out, Release{
			TagName:     t.Name,
			Name:        t.Name,
			HTMLURL:     fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, t.Name),
		})
	}
	return out, nil
}

func githubAPIError(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return fmt.Errorf("token GitHub invalide ou expiré (401) — vérifiez GITHUB_TOKEN dans les paramètres")
	case http.StatusForbidden:
		return fmt.Errorf("limite de taux GitHub atteinte (403) — configurez un GITHUB_TOKEN pour augmenter la limite")
	case http.StatusNotFound:
		return fmt.Errorf("dépôt introuvable sur GitHub (404) — vérifiez owner/repo")
	default:
		return fmt.Errorf("erreur GitHub API (%d)", status)
	}
}
