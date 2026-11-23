package releasetracker

import (
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
)

// newGitClient builds the provider client for a tracker. GitHub always uses
// the global GITHUB_TOKEN. GitLab and Gitea use the tracker's own instance URL
// and token, never the GitHub one: a token must only ever travel to the host it
// was issued for.
func newGitClient(cfg *config.Config, t models.ReleaseTracker) gitprovider.Client {
	if t.Provider == "gitlab" || t.Provider == "gitea" {
		return gitprovider.NewClient(t.Provider, t.GitToken, t.GitBaseURL)
	}
	return gitprovider.NewClient(t.Provider, cfg.GitHubToken, "")
}

// normalizeGitInstance validates and canonicalizes the self-hosted instance
// fields of a tracker that names a repository. GitHub (and a docker tracker
// without a git link) carries no instance data: both fields are reset and the
// stored token is erased so that switching provider never leaves a stale
// secret behind. Returns a client-facing error, nil when valid.
func normalizeGitInstance(m *models.ReleaseTracker) *apperr.Error {
	selfHostable := (m.Provider == "gitlab" || m.Provider == "gitea") &&
		m.RepoOwner != "" && m.RepoName != ""
	if !selfHostable {
		if m.GitBaseURL != "" || m.GitToken != "" || m.Provider == "github" || m.Provider == "" {
			m.GitBaseURL = ""
			m.GitToken = ""
			m.ClearGitToken = true
		}
		return nil
	}
	base, err := gitprovider.NormalizeBaseURL(m.GitBaseURL)
	if err != nil {
		return apperr.Validation("invalid git instance URL: "+err.Error()).I18n(apperr.CodeInvalidGitBaseURL, nil)
	}
	m.GitBaseURL = base
	m.GitToken = strings.TrimSpace(m.GitToken)
	return nil
}
