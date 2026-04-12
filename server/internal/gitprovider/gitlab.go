package gitprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

type gitLabClient struct {
	authToken string
	client    *http.Client
}

func newGitLabClient(authToken string) *gitLabClient {
	return &gitLabClient{
		authToken: authToken,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchLatestRelease returns the latest GitLab release with fallback to tags
func (c *gitLabClient) FetchLatestRelease(owner, repo string) (string, string, string, error) {
	release, err := c.fetchGitLabRelease(owner, repo)
	if err != nil {
		return "", "", "", err
	}
	if release.TagName != "" {
		return release.TagName, release.HTMLURL, release.Name, nil
	}

	// Fallback to tags
	tag, err := c.fetchGitLabTag(owner, repo)
	if err != nil {
		return "", "", "", err
	}
	return tag.TagName, tag.HTMLURL, tag.Name, nil
}

// FetchReleaseHistory returns recent GitLab releases with fallback to tags.
func (c *gitLabClient) FetchReleaseHistory(owner, repo string, limit int) ([]Release, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	projectID := fmt.Sprintf("%s%%2F%s", owner, repo)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/releases?per_page=%d", projectID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("PRIVATE-TOKEN", c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return c.fetchGitLabTagHistory(owner, repo, limit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var releases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		PublishedAt time.Time `json:"published_at"`
		WebURL      string    `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return c.fetchGitLabTagHistory(owner, repo, limit)
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
			HTMLURL:     r.WebURL,
		})
	}
	return out, nil
}

// FetchDockerManifestDigest returns the SHA256 digest from Docker registries
func (c *gitLabClient) FetchDockerManifestDigest(imageName, tag string) (string, error) {
	return fetchDockerManifestDigest(c.client, imageName, tag)
}

// FetchDockerVersionForDigest finds a versioned tag matching the given digest.
func (c *gitLabClient) FetchDockerVersionForDigest(imageName, digest string) string {
	return fetchDockerVersionForDigest(c.client, imageName, digest)
}

func (c *gitLabClient) fetchGitLabRelease(owner, repo string) (*models.GitHubRelease, error) {
	// GitLab API: GET /projects/:id/releases
	projectID := fmt.Sprintf("%s%%2F%s", owner, repo)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/releases?per_page=1", projectID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("PRIVATE-TOKEN", c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return &models.GitHubRelease{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var releases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		PublishedAt time.Time `json:"published_at"`
		WebURL      string    `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return &models.GitHubRelease{}, nil
	}

	return &models.GitHubRelease{
		TagName:     releases[0].TagName,
		Name:        releases[0].Name,
		PublishedAt: releases[0].PublishedAt,
		HTMLURL:     releases[0].WebURL,
	}, nil
}

func (c *gitLabClient) fetchGitLabTag(owner, repo string) (*models.GitHubRelease, error) {
	projectID := fmt.Sprintf("%s%%2F%s", owner, repo)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/tags?per_page=1", projectID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("PRIVATE-TOKEN", c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var tags []struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return &models.GitHubRelease{}, nil
	}

	return &models.GitHubRelease{
		TagName: tags[0].Name,
		HTMLURL: tags[0].WebURL,
	}, nil
}

func (c *gitLabClient) fetchGitLabTagHistory(owner, repo string, limit int) ([]Release, error) {
	projectID := fmt.Sprintf("%s%%2F%s", owner, repo)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/tags?per_page=%d", projectID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if c.authToken != "" {
		req.Header.Set("PRIVATE-TOKEN", c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var tags []struct {
		Name   string `json:"name"`
		WebURL string `json:"web_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	out := make([]Release, 0, len(tags))
	for _, t := range tags {
		out = append(out, Release{
			TagName:     t.Name,
			Name:        t.Name,
			HTMLURL:     t.WebURL,
		})
	}
	return out, nil
}
