package gitprovider

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func gitlabServer(t *testing.T, h http.HandlerFunc) (*gitLabClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newGitLabClient(srv.URL+"/gitlab", "glpat-secret", srv.Client()), srv
}

func TestGitLabLatestReleaseUsesBaseURLTokenAndEncodedProject(t *testing.T) {
	var gotPath, gotRawPath, gotToken, gotQuery string
	c, srv := gitlabServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotRawPath, gotToken, gotQuery = r.URL.Path, r.URL.EscapedPath(), r.Header.Get("PRIVATE-TOKEN"), r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"tag_name":"v2.1.0","name":"Two one","released_at":"2026-01-02T03:04:05Z","_links":{"self":"https://git.lan/grp/sub/app/-/releases/v2.1.0"}}]`))
	})
	_ = srv

	tag, link, name, err := c.FetchLatestRelease("grp/sub", "app")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v2.1.0" || name != "Two one" || link != "https://git.lan/grp/sub/app/-/releases/v2.1.0" {
		t.Fatalf("unexpected release %q %q %q", tag, link, name)
	}
	if gotPath != "/gitlab/api/v4/projects/grp/sub/app/releases" || gotRawPath != "/gitlab/api/v4/projects/grp%2Fsub%2Fapp/releases" {
		t.Errorf("project path not encoded as one segment: %q (raw %q)", gotPath, gotRawPath)
	}
	if gotToken != "glpat-secret" {
		t.Errorf("PRIVATE-TOKEN = %q", gotToken)
	}
	if gotQuery != "per_page=1" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestGitLabHistoryMapsReleasesAndLimit(t *testing.T) {
	var gotQuery string
	c, _ := gitlabServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"tag_name":"v2","name":"Two","released_at":"2026-02-01T00:00:00Z","upcoming_release":true},{"tag_name":"v1","name":"One","released_at":"2026-01-01T00:00:00Z"}]`))
	})
	got, err := c.FetchReleaseHistory("o", "r", 500)
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "per_page=50" {
		t.Errorf("limit must be clamped to 50, query = %q", gotQuery)
	}
	if len(got) != 2 || got[0].TagName != "v2" || !got[0].Prerelease || got[1].Prerelease {
		t.Fatalf("unexpected history %+v", got)
	}
	// No `_links.self`: the link is built from the instance base URL.
	if want := c.api.baseURL + "/o/r/-/releases/v2"; got[0].HTMLURL != want {
		t.Errorf("fallback release URL = %q, want %q", got[0].HTMLURL, want)
	}
	if got[0].PublishedAt.IsZero() || got[0].PublishedAt.Year() != 2026 {
		t.Errorf("released_at not mapped: %v", got[0].PublishedAt)
	}
}

func TestGitLabFallsBackToTagsWhenNoRelease(t *testing.T) {
	for name, releasesResp := range map[string]func(http.ResponseWriter){
		"empty list": func(w http.ResponseWriter) { _, _ = w.Write([]byte(`[]`)) },
		"404":        func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) },
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := gitlabServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/gitlab/api/v4/projects/o%2Fr/repository/tags" || r.URL.EscapedPath() == "/gitlab/api/v4/projects/o%2Fr/repository/tags" {
					_, _ = w.Write([]byte(`[{"name":"v0.9"},{"name":"v0.8"}]`))
					return
				}
				releasesResp(w)
			})
			tag, link, _, err := c.FetchLatestRelease("o", "r")
			if err != nil {
				t.Fatal(err)
			}
			if tag != "v0.9" || link != c.api.baseURL+"/o/r/-/tags/v0.9" {
				t.Fatalf("tag fallback wrong: %q %q", tag, link)
			}
		})
	}
}

func TestGitLabNoReleaseNoTagIsEmptyNotError(t *testing.T) {
	c, _ := gitlabServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	tag, _, _, err := c.FetchLatestRelease("o", "r")
	if err != nil || tag != "" {
		t.Fatalf("expected empty result without error, got %q, %v", tag, err)
	}
}

func TestGitLabAPIErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		c, _ := gitlabServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		_, _, _, err := c.FetchLatestRelease("o", "r")
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != status || apiErr.Provider != "GitLab" {
			t.Errorf("status %d: got %v", status, err)
		}
	}
}

func TestGitLabMalformedBody(t *testing.T) {
	c, _ := gitlabServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`not json`)) })
	if _, err := c.FetchReleaseHistory("o", "r", 5); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestGitLabTagFallbackAuthFailureSurfaces(t *testing.T) {
	c, _ := gitlabServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "" && r.URL.EscapedPath() == "/gitlab/api/v4/projects/o%2Fr/releases" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.FetchReleaseHistory("o", "r", 5)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("got %v", err)
	}
}
