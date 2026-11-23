package gitprovider

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type giteaClient struct {
	api *restAPI
}

// newGiteaClient targets baseURL (default codeberg.org). Works for Forgejo too,
// which exposes the same /api/v1 surface. The token is sent as
// "Authorization: token <t>".
func newGiteaClient(baseURL, authToken string, httpClient *http.Client) *giteaClient {
	if baseURL == "" {
		baseURL = DefaultGiteaBaseURL
	}
	return &giteaClient{api: &restAPI{
		provider:   "Gitea",
		baseURL:    baseURL,
		token:      authToken,
		authHeader: func(t string) (string, string) { return "Authorization", "token " + t },
		http:       httpClient,
	}}
}

func (c *giteaClient) repoPath(owner, repo string) string {
	return "/api/v1/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

func (c *giteaClient) tagURL(owner, repo, tag string) string {
	return fmt.Sprintf("%s/%s/%s/releases/tag/%s", c.api.baseURL, owner, repo, url.PathEscape(tag))
}

type giteaRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// FetchLatestRelease returns the latest published (non-draft, non-prerelease)
// Gitea release, falling back to the newest tag. An empty tag means the
// repository has neither.
func (c *giteaClient) FetchLatestRelease(owner, repo string) (string, string, string, error) {
	var r giteaRelease
	err := c.api.getJSON(c.repoPath(owner, repo)+"/releases/latest", &r)
	if err == nil && r.TagName != "" {
		return r.TagName, r.HTMLURL, r.Name, nil
	}
	if err != nil && !isNotFound(err) {
		return "", "", "", err
	}
	tags, err := c.fetchTagHistory(owner, repo, 1)
	if err != nil {
		return "", "", "", err
	}
	if len(tags) == 0 {
		return "", "", "", nil
	}
	return tags[0].TagName, tags[0].HTMLURL, tags[0].Name, nil
}

// FetchReleaseHistory returns recent releases (newest first), or tags when the
// repository publishes no release.
func (c *giteaClient) FetchReleaseHistory(owner, repo string, limit int) ([]Release, error) {
	limit = clampLimit(limit)
	var releases []giteaRelease
	err := c.api.getJSON(fmt.Sprintf("%s/releases?limit=%d", c.repoPath(owner, repo), limit), &releases)
	if err != nil {
		if isNotFound(err) {
			return c.fetchTagHistory(owner, repo, limit)
		}
		return nil, err
	}
	if len(releases) == 0 {
		return c.fetchTagHistory(owner, repo, limit)
	}
	out := make([]Release, 0, len(releases))
	for _, r := range releases {
		published := r.PublishedAt
		if published.IsZero() {
			published = time.Now().UTC()
		}
		out = append(out, Release{
			TagName:     r.TagName,
			Name:        r.Name,
			PublishedAt: published,
			HTMLURL:     r.HTMLURL,
			Prerelease:  r.Prerelease,
			Draft:       r.Draft,
		})
	}
	return out, nil
}

func (c *giteaClient) fetchTagHistory(owner, repo string, limit int) ([]Release, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := c.api.getJSON(fmt.Sprintf("%s/tags?limit=%d", c.repoPath(owner, repo), limit), &tags); err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(tags))
	for _, t := range tags {
		out = append(out, Release{TagName: t.Name, Name: t.Name, HTMLURL: c.tagURL(owner, repo, t.Name)})
	}
	return out, nil
}

// FetchDockerManifestDigest returns the SHA256 digest from Docker registries.
func (c *giteaClient) FetchDockerManifestDigest(imageName, tag string) (string, error) {
	return fetchDockerManifestDigest(c.api.http, imageName, tag, regCreds{token: c.api.token})
}

// FetchDockerVersionForDigest finds a versioned tag matching the given digest.
func (c *giteaClient) FetchDockerVersionForDigest(imageName, digest string) string {
	return fetchDockerVersionForDigest(c.api.http, imageName, digest, regCreds{token: c.api.token})
}
