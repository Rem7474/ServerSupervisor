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

// FetchDockerManifestDigest returns the SHA256 digest from Docker registries
func (c *gitLabClient) FetchDockerManifestDigest(imageName, tag string) (string, error) {
	return fetchDockerManifestDigest(c.client, imageName, tag)
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
