package github

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type Tracker struct {
	db     *database.DB
	cfg    *config.Config
	client *http.Client
	stop   chan struct{}
}

func NewTracker(db *database.DB, cfg *config.Config) *Tracker {
	return &Tracker{
		db:  db,
		cfg: cfg,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		stop: make(chan struct{}),
	}
}

// Start begins periodic polling of GitHub releases
func (t *Tracker) Start() {
	log.Printf("GitHub release tracker started (poll interval: %v)", t.cfg.GitHubPollInterval)

	// Initial check on startup
	go t.checkAllRepos()

	ticker := time.NewTicker(t.cfg.GitHubPollInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				t.checkAllRepos()
			case <-t.stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (t *Tracker) Stop() {
	close(t.stop)
}

func (t *Tracker) checkAllRepos() {
	repos, err := t.db.GetTrackedRepos()
	if err != nil {
		log.Printf("GitHub tracker: failed to fetch repos: %v", err)
		return
	}

	for _, repo := range repos {
		release, err := t.getLatestRelease(repo.Owner, repo.Repo)
		if err != nil {
			log.Printf("GitHub tracker: failed to check %s/%s: %v", repo.Owner, repo.Repo, err)
			continue
		}

		if release.TagName != repo.LatestVersion && release.TagName != "" {
			log.Printf("GitHub tracker: new release for %s/%s: %s (was %s)",
				repo.Owner, repo.Repo, release.TagName, repo.LatestVersion)

			t.db.UpdateTrackedRepo(repo.ID, release.TagName, release.HTMLURL, release.PublishedAt)
		}
	}
}

func (t *Tracker) getLatestRelease(owner, repo string) (*models.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if t.cfg.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.cfg.GitHubToken)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Try tags instead (some repos don't use releases)
		return t.getLatestTag(owner, repo)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release models.GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (t *Tracker) getLatestTag(owner, repo string) (*models.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=1", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if t.cfg.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.cfg.GitHubToken)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
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
