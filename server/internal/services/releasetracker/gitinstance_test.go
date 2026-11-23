package releasetracker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
)

func gitReq(provider, base, token string) models.ReleaseTrackerRequest {
	return models.ReleaseTrackerRequest{
		Name: "t", TrackerType: "git", Provider: provider, RepoOwner: "grp", RepoName: "app",
		GitBaseURL: base, GitToken: token,
	}
}

func TestCreate_SelfHostedInstanceIsNormalizedAndPersisted(t *testing.T) {
	for _, provider := range []string{"gitlab", "gitea"} {
		t.Run(provider, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := newSvc(repo).Create(context.Background(), gitReq(provider, " https://git.lan:8443/forge/ ", " tok "))
			if err != nil {
				t.Fatalf("expected a valid tracker, got %v", err)
			}
			got := repo.lastCreated
			if got.GitBaseURL != "https://git.lan:8443/forge" {
				t.Errorf("base URL not normalized: %q", got.GitBaseURL)
			}
			if got.GitToken != "tok" {
				t.Errorf("token not trimmed/persisted: %q", got.GitToken)
			}
		})
	}
}

func TestCreate_SelfHostedWithoutBaseURLUsesPublicInstance(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newSvc(repo).Create(context.Background(), gitReq("gitlab", "", "")); err != nil {
		t.Fatal(err)
	}
	if repo.lastCreated.GitBaseURL != "" {
		t.Errorf("empty base URL must stay empty (resolved to the public instance at poll time), got %q", repo.lastCreated.GitBaseURL)
	}
}

func TestCreate_RejectsUnsafeBaseURL(t *testing.T) {
	bad := []string{
		"ftp://git.lan", "file:///etc/passwd", "git.lan", "http://localhost:3000",
		"http://127.0.0.1", "http://169.254.169.254/", "https://" + "u:p" + "@git.lan", "https://git.lan?x=1",
	}
	for _, base := range bad {
		t.Run(base, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := newSvc(repo).Create(context.Background(), gitReq("gitea", base, "tok"))
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.HTTPStatus != http.StatusBadRequest {
				t.Fatalf("expected a 400, got %v", err)
			}
			if ae.I18nKey != apperr.CodeInvalidGitBaseURL {
				t.Errorf("i18n key = %q", ae.I18nKey)
			}
			if repo.created {
				t.Error("must not persist a tracker with an unsafe base URL")
			}
		})
	}
}

func TestCreate_AllowsPrivateNetworkInstances(t *testing.T) {
	for _, base := range []string{"http://192.168.1.50:3000", "http://10.0.0.7", "https://gitea.lan", "http://[fd00::5]:3000"} {
		repo := &fakeRepo{}
		if _, err := newSvc(repo).Create(context.Background(), gitReq("gitea", base, "")); err != nil {
			t.Errorf("%s: self-hosted instances on a private network must be accepted, got %v", base, err)
		}
	}
}

func TestCreate_GitHubDropsInstanceFields(t *testing.T) {
	repo := &fakeRepo{}
	if _, err := newSvc(repo).Create(context.Background(), gitReq("github", "https://git.lan", "tok")); err != nil {
		t.Fatal(err)
	}
	got := repo.lastCreated
	if got.GitBaseURL != "" || got.GitToken != "" {
		t.Errorf("github tracker must not store instance data: %q / %q", got.GitBaseURL, got.GitToken)
	}
}

func TestCreate_DockerTrackerWithoutGitLinkDropsInstanceFields(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).Create(context.Background(), models.ReleaseTrackerRequest{
		Name: "d", TrackerType: "docker", DockerImage: "nginx", GitBaseURL: "https://git.lan", GitToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastCreated.GitBaseURL != "" || repo.lastCreated.GitToken != "" {
		t.Error("no git link: instance data must be dropped")
	}
}

func TestCreate_DockerTrackerGitLinkOnSelfHostedInstance(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).Create(context.Background(), models.ReleaseTrackerRequest{
		Name: "d", TrackerType: "docker", DockerImage: "nginx", Provider: "gitea", RepoOwner: "o", RepoName: "r",
		GitBaseURL: "https://git.lan/", GitToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.lastCreated.GitBaseURL != "https://git.lan" || repo.lastCreated.GitToken != "tok" {
		t.Errorf("docker tracker's git link must keep its instance: %+v", repo.lastCreated)
	}
}

func TestUpdate_TokenSemantics(t *testing.T) {
	t.Run("empty token keeps the stored one", func(t *testing.T) {
		repo := &fakeRepo{}
		if err := newSvc(repo).Update(context.Background(), "id", gitReq("gitlab", "https://git.lan", "")); err != nil {
			t.Fatal(err)
		}
		if repo.lastUpdated.GitToken != "" || repo.lastUpdated.ClearGitToken {
			t.Errorf("an empty token must neither overwrite nor clear: %+v", repo.lastUpdated)
		}
	})
	t.Run("explicit clear", func(t *testing.T) {
		repo := &fakeRepo{}
		req := gitReq("gitlab", "https://git.lan", "")
		req.ClearGitToken = true
		if err := newSvc(repo).Update(context.Background(), "id", req); err != nil {
			t.Fatal(err)
		}
		if !repo.lastUpdated.ClearGitToken {
			t.Error("clear_git_token must reach the repository")
		}
	})
	t.Run("switching to github erases the stored token", func(t *testing.T) {
		repo := &fakeRepo{}
		if err := newSvc(repo).Update(context.Background(), "id", gitReq("github", "", "")); err != nil {
			t.Fatal(err)
		}
		if !repo.lastUpdated.ClearGitToken || repo.lastUpdated.GitBaseURL != "" {
			t.Errorf("provider switch must wipe instance data: %+v", repo.lastUpdated)
		}
	})
	t.Run("unsafe URL rejected on update", func(t *testing.T) {
		err := newSvc(&fakeRepo{}).Update(context.Background(), "id", gitReq("gitea", "http://127.0.0.1:3000", ""))
		if status(err) != 400 {
			t.Fatalf("expected 400, got %v", err)
		}
	})
}

func TestTrackerJSONNeverExposesTheToken(t *testing.T) {
	tr := models.ReleaseTracker{GitToken: "super-secret", HasGitToken: true, GitBaseURL: "https://git.lan"}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "super-secret") || strings.Contains(string(b), `"git_token"`) {
		t.Errorf("token leaked in API payload: %s", b)
	}
	if !strings.Contains(string(b), `"has_git_token":true`) {
		t.Errorf("has_git_token missing: %s", b)
	}
}

func TestGitProviderErrorNamesTheProvider(t *testing.T) {
	cases := []struct {
		status   int
		provider string
		key      string
	}{
		{401, "GitLab", apperr.CodeGitProviderUnauthorized},
		{403, "Gitea", apperr.CodeGitProviderRateLimited},
		{404, "Gitea", apperr.CodeGitProviderNotFound},
		{500, "GitLab", apperr.CodeGitProviderError},
	}
	for _, tc := range cases {
		aerr := gitProviderHTTPError(&gitprovider.APIError{Status: tc.status, Provider: tc.provider})
		if aerr.I18nKey != tc.key {
			t.Errorf("status %d: key %q, want %q", tc.status, aerr.I18nKey, tc.key)
		}
		if !strings.Contains(aerr.Message, tc.provider) {
			t.Errorf("status %d: message %q must name %s", tc.status, aerr.Message, tc.provider)
		}
		if poll := gitProviderPollError(&gitprovider.APIError{Status: tc.status, Provider: tc.provider}); !strings.Contains(poll, tc.provider) {
			t.Errorf("status %d: poll error %q must name %s", tc.status, poll, tc.provider)
		}
	}
	// Legacy APIError without provider keeps reading as GitHub.
	if aerr := gitProviderHTTPError(&gitprovider.APIError{Status: 404}); !strings.Contains(aerr.Message, "GitHub") {
		t.Errorf("provider-less error must default to GitHub, got %q", aerr.Message)
	}
}
