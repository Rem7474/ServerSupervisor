package gitprovider

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	ok := map[string]string{
		"":                               "",
		"  ":                             "",
		"https://git.example.com":        "https://git.example.com",
		"https://git.example.com/":       "https://git.example.com",
		"http://192.168.1.20:3000/":      "http://192.168.1.20:3000",
		"https://example.com/gitlab//":   "https://example.com/gitlab",
		"https://git.lan":                "https://git.lan",
		"http://10.0.0.5":                "http://10.0.0.5",
		"https://[fd00::1]:8443/forge/":  "https://[fd00::1]:8443/forge",
		" https://git.example.com/sub/ ": "https://git.example.com/sub",
	}
	for in, want := range ok {
		got, err := NormalizeBaseURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeBaseURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	bad := []string{
		"ftp://git.example.com",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"git.example.com",
		"https://",
		"https://" + "user:pass" + "@git.example.com",
		"https://git.example.com?x=1",
		"https://git.example.com#frag",
		"http://localhost:3000",
		"http://gitea.localhost",
		"http://127.0.0.1:3000",
		"http://[::1]:3000",
		"http://169.254.169.254/latest",
		"http://[fe80::1]",
		"http://0.0.0.0",
		"http://[::ffff:127.0.0.1]",
		"http://224.0.0.1",
	}
	for _, in := range bad {
		if got, err := NormalizeBaseURL(in); err == nil {
			t.Errorf("NormalizeBaseURL(%q) = %q, want an error", in, got)
		}
	}
}

func TestGuardedClientRefusesLoopbackAtDialTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request must never reach the loopback server")
	}))
	defer srv.Close()

	c := NewClient("gitea", "tok", srv.URL)
	_, _, _, err := c.FetchLatestRelease("o", "r")
	if err == nil || !strings.Contains(err.Error(), errBlockedAddress.Error()) {
		t.Fatalf("expected the dial guard to refuse the connection, got %v", err)
	}
}

func TestGuardedClientDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("redirect target must not be contacted")
	}))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	// A plain client with the guarded client's redirect policy.
	hc := newGuardedHTTPClient()
	hc.Transport = http.DefaultTransport
	c := newGiteaClient(srv.URL, "tok", hc)
	_, err := c.FetchReleaseHistory("o", "r", 5)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusFound {
		t.Fatalf("expected an APIError with status 302, got %v", err)
	}
}

func TestNewClientDefaultsToPublicInstances(t *testing.T) {
	if got := NewClient("gitlab", "", "").(*gitLabClient).api.baseURL; got != DefaultGitLabBaseURL {
		t.Errorf("gitlab default base = %q", got)
	}
	if got := NewClient("gitea", "", "").(*giteaClient).api.baseURL; got != DefaultGiteaBaseURL {
		t.Errorf("gitea default base = %q", got)
	}
	if _, ok := NewClient("github", "", "").(*gitHubClient); !ok {
		t.Error("github provider must yield the github client")
	}
}
