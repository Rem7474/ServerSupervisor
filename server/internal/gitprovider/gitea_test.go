package gitprovider

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func giteaServer(t *testing.T, h http.HandlerFunc) *giteaClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return newGiteaClient(srv.URL+"/forge", "gt-secret", srv.Client())
}

func TestGiteaLatestReleaseUsesBaseURLAndToken(t *testing.T) {
	var gotPath, gotAuth string
	c := giteaServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"tag_name":"v3.0.0","name":"Three","html_url":"https://git.lan/o/r/releases/tag/v3.0.0"}`))
	})
	tag, link, name, err := c.FetchLatestRelease("o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v3.0.0" || name != "Three" || link != "https://git.lan/o/r/releases/tag/v3.0.0" {
		t.Fatalf("unexpected release %q %q %q", tag, link, name)
	}
	if gotPath != "/forge/api/v1/repos/o/r/releases/latest" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "token gt-secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestGiteaNoTokenSendsNoAuthHeader(t *testing.T) {
	var hasAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth = r.Header["Authorization"]
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := newGiteaClient(srv.URL, "", srv.Client())
	if _, err := c.FetchReleaseHistory("o", "r", 3); err != nil {
		t.Fatal(err)
	}
	if hasAuth {
		t.Error("no Authorization header expected without a token")
	}
}

func TestGiteaHistoryMapsReleasesAndLimit(t *testing.T) {
	var gotQuery string
	c := giteaServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"tag_name":"v2-rc1","name":"RC","published_at":"2026-02-01T00:00:00Z","html_url":"u2","prerelease":true},{"tag_name":"v1","name":"One","published_at":"2026-01-01T00:00:00Z","html_url":"u1","draft":true}]`))
	})
	got, err := c.FetchReleaseHistory("o", "r", 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "limit=20" {
		t.Errorf("default limit expected, query = %q", gotQuery)
	}
	if len(got) != 2 || !got[0].Prerelease || !got[1].Draft || got[0].HTMLURL != "u2" {
		t.Fatalf("unexpected history %+v", got)
	}
}

func giteaTagsFallbackServer(releases func(http.ResponseWriter)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/forge/api/v1/repos/o/r/tags" {
			_, _ = w.Write([]byte(`[{"name":"v0.5"},{"name":"v0.4"}]`))
			return
		}
		releases(w)
	}
}

func TestGiteaLatestFallsBackToTagsOn404(t *testing.T) {
	c := giteaServer(t, giteaTagsFallbackServer(func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) }))
	tag, link, _, err := c.FetchLatestRelease("o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.5" || link != c.api.baseURL+"/o/r/releases/tag/v0.5" {
		t.Fatalf("tag fallback wrong: %q %q", tag, link)
	}
}

func TestGiteaHistoryFallsBackToTags(t *testing.T) {
	for name, releases := range map[string]func(http.ResponseWriter){
		"empty": func(w http.ResponseWriter) { _, _ = w.Write([]byte(`[]`)) },
		"404":   func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) },
	} {
		t.Run(name, func(t *testing.T) {
			c := giteaServer(t, giteaTagsFallbackServer(releases))
			got, err := c.FetchReleaseHistory("o", "r", 10)
			if err != nil || len(got) != 2 || got[1].TagName != "v0.4" {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestGiteaNoReleaseNoTagIsEmptyNotError(t *testing.T) {
	c := giteaServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/forge/api/v1/repos/o/r/releases/latest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	tag, _, _, err := c.FetchLatestRelease("o", "r")
	if err != nil || tag != "" {
		t.Fatalf("expected empty result without error, got %q, %v", tag, err)
	}
}

func TestGiteaAPIErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway} {
		c := giteaServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		for name, call := range map[string]func() error{
			"latest":  func() error { _, _, _, err := c.FetchLatestRelease("o", "r"); return err },
			"history": func() error { _, err := c.FetchReleaseHistory("o", "r", 5); return err },
		} {
			var apiErr *APIError
			if err := call(); !errors.As(err, &apiErr) || apiErr.Status != status || apiErr.Provider != "Gitea" {
				t.Errorf("%s status %d: got %v", name, status, err)
			}
		}
	}
}

func TestGiteaUnreachableInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()
	c := newGiteaClient(url, "", http.DefaultClient)
	if _, _, _, err := c.FetchLatestRelease("o", "r"); err == nil {
		t.Fatal("expected a connection error")
	}
}
