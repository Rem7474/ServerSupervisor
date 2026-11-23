package gitprovider

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type gitLabClient struct {
	api *restAPI
}

// newGitLabClient targets baseURL (default gitlab.com). The token is sent as
// PRIVATE-TOKEN, which accepts personal, project and group access tokens.
func newGitLabClient(baseURL, authToken string, httpClient *http.Client) *gitLabClient {
	if baseURL == "" {
		baseURL = DefaultGitLabBaseURL
	}
	return &gitLabClient{api: &restAPI{
		provider:   "GitLab",
		baseURL:    baseURL,
		token:      authToken,
		authHeader: func(t string) (string, string) { return "PRIVATE-TOKEN", t },
		http:       httpClient,
	}}
}

func (c *gitLabClient) projectPath(owner, repo string) string {
	return "/api/v4/projects/" + url.PathEscape(owner+"/"+repo)
}

func (c *gitLabClient) webBase(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s", c.api.baseURL, owner, repo)
}

type gitLabRelease struct {
	TagName         string    `json:"tag_name"`
	Name            string    `json:"name"`
	ReleasedAt      time.Time `json:"released_at"`
	UpcomingRelease bool      `json:"upcoming_release"`
	Links           struct {
		Self string `json:"self"`
	} `json:"_links"`
}

// FetchLatestRelease returns the newest GitLab release, falling back to the
// newest tag. An empty tag means the project has neither.
func (c *gitLabClient) FetchLatestRelease(owner, repo string) (string, string, string, error) {
	history, err := c.FetchReleaseHistory(owner, repo, 1)
	if err != nil {
		return "", "", "", err
	}
	if len(history) == 0 {
		return "", "", "", nil
	}
	return history[0].TagName, history[0].HTMLURL, history[0].Name, nil
}

// FetchReleaseHistory returns recent releases (newest first), or tags when the
// project publishes no release or its releases are not readable (404).
func (c *gitLabClient) FetchReleaseHistory(owner, repo string, limit int) ([]Release, error) {
	limit = clampLimit(limit)
	var releases []gitLabRelease
	err := c.api.getJSON(fmt.Sprintf("%s/releases?per_page=%d", c.projectPath(owner, repo), limit), &releases)
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
		link := r.Links.Self
		if link == "" {
			link = fmt.Sprintf("%s/-/releases/%s", c.webBase(owner, repo), url.PathEscape(r.TagName))
		}
		published := r.ReleasedAt
		if published.IsZero() {
			published = time.Now().UTC()
		}
		out = append(out, Release{
			TagName:     r.TagName,
			Name:        r.Name,
			PublishedAt: published,
			HTMLURL:     link,
			Prerelease:  r.UpcomingRelease,
		})
	}
	return out, nil
}

func (c *gitLabClient) fetchTagHistory(owner, repo string, limit int) ([]Release, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := c.api.getJSON(fmt.Sprintf("%s/repository/tags?per_page=%d", c.projectPath(owner, repo), limit), &tags); err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(tags))
	for _, t := range tags {
		out = append(out, Release{
			TagName: t.Name,
			Name:    t.Name,
			HTMLURL: fmt.Sprintf("%s/-/tags/%s", c.webBase(owner, repo), url.PathEscape(t.Name)),
		})
	}
	return out, nil
}

// FetchDockerManifestDigest returns the SHA256 digest from Docker registries.
func (c *gitLabClient) FetchDockerManifestDigest(imageName, tag string) (string, error) {
	return fetchDockerManifestDigest(c.api.http, imageName, tag, regCreds{token: c.api.token})
}

// FetchDockerVersionForDigest finds a versioned tag matching the given digest.
func (c *gitLabClient) FetchDockerVersionForDigest(imageName, digest string) string {
	return fetchDockerVersionForDigest(c.api.http, imageName, digest, regCreds{token: c.api.token})
}
