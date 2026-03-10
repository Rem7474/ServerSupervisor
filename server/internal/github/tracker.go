package github

import (
	"log"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/gitprovider"
)

type Tracker struct {
	db   *database.DB
	cfg  *config.Config
	stop chan struct{}
}

func NewTracker(db *database.DB, cfg *config.Config) *Tracker {
	return &Tracker{
		db:   db,
		cfg:  cfg,
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

	client := gitprovider.NewClient("github", t.cfg.GitHubToken)

	for _, repo := range repos {
		tag, htmlURL, _, err := client.FetchLatestRelease(repo.Owner, repo.Repo)
		if err != nil {
			log.Printf("GitHub tracker: failed to check %s/%s: %v", repo.Owner, repo.Repo, err)
			continue
		}

		if tag != repo.LatestVersion && tag != "" {
			log.Printf("GitHub tracker: new release for %s/%s: %s (was %s)",
				repo.Owner, repo.Repo, tag, repo.LatestVersion)
			_ = t.db.UpdateTrackedRepo(repo.ID, tag, htmlURL, time.Time{})
		}
	}
}