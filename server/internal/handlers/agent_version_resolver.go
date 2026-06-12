package handlers

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/gitprovider"
)

const (
	defaultLatestAgentVersion = "7.6.5"
	agentReleaseOwner         = "Rem7474"
	agentReleaseRepo          = "ServerSupervisor"
	agentReleaseCacheDuration = 10 * time.Minute
)

var latestAgentVersionCache struct {
	mu        sync.Mutex
	version   string
	refreshed time.Time
}

func ResolveLatestAgentVersion(cfg *config.Config) string {
	latestAgentVersionCache.mu.Lock()
	defer latestAgentVersionCache.mu.Unlock()

	now := time.Now()
	if latestAgentVersionCache.version != "" && now.Sub(latestAgentVersionCache.refreshed) < agentReleaseCacheDuration {
		return latestAgentVersionCache.version
	}

	client := gitprovider.NewClient("github", cfg.GitHubToken)
	tag, _, _, err := client.FetchLatestRelease(agentReleaseOwner, agentReleaseRepo)
	if err != nil {
		if latestAgentVersionCache.version != "" {
			slog.Warn("agent version resolver: failed to refresh latest release, using cached value", slog.Any("err", err), slog.String("cached", latestAgentVersionCache.version))
			return latestAgentVersionCache.version
		}
		slog.Warn("agent version resolver: failed to refresh latest release, using default", slog.Any("err", err), slog.String("default", defaultLatestAgentVersion))
		latestAgentVersionCache.version = defaultLatestAgentVersion
		latestAgentVersionCache.refreshed = now
		return latestAgentVersionCache.version
	}

	resolved := normalizeReleaseTag(tag)
	if resolved == "" {
		if latestAgentVersionCache.version != "" {
			slog.Warn("agent version resolver: empty tag from provider, using cached value", slog.String("cached", latestAgentVersionCache.version))
			return latestAgentVersionCache.version
		}
		latestAgentVersionCache.version = defaultLatestAgentVersion
		latestAgentVersionCache.refreshed = now
		return latestAgentVersionCache.version
	}

	latestAgentVersionCache.version = resolved
	latestAgentVersionCache.refreshed = now
	return latestAgentVersionCache.version
}

func normalizeReleaseTag(tag string) string {
	t := strings.TrimSpace(tag)
	t = strings.TrimPrefix(t, "v")
	t = strings.TrimPrefix(t, "V")
	return t
}
