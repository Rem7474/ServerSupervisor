// Package dashboard aggregates cross-domain "needs attention" signals for the
// dashboard and navbar. Read-only: it owns no writes and no table of its own.
package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

const (
	sslExpiryWarningDays           = 14
	neverConnectedToleranceSeconds = 60
)

// Repository is the data-access port the service depends on. *database.DB
// satisfies it structurally; tests provide an in-memory fake. Every method
// here already exists on *database.DB, reused as-is from the host/proxmox/
// npm/releasetracker/ssl domains rather than re-derived — see each item's
// comment in Attention() for the exact frontend logic being ported.
type Repository interface {
	GetAllHosts(ctx context.Context) ([]models.Host, error)
	ListProxmoxGuestLinks(ctx context.Context, status string) ([]models.ProxmoxGuestLink, error)
	ListAllNPMProxyHostsEnriched(ctx context.Context) ([]models.NPMProxyHostEnriched, error)
	ListReleaseTrackers(ctx context.Context) ([]models.ReleaseTracker, error)
	TrackerDriftDetected(ctx context.Context, t models.ReleaseTracker) (bool, error)
	ListSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error)
}

// Service holds the dashboard-attention use-case.
type Service struct {
	repo Repository
}

// NewService wires the service. repo is normally *database.DB.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func plural(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

// isNeverConnectedHost mirrors frontend/src/utils/hosts.ts's isNeverConnectedHost:
// a host whose last_seen is still within a few seconds of its created_at has
// never actually reported in (RegisterHost stamps last_seen=now() at
// creation; only a real agent report moves it meaningfully later).
func isNeverConnectedHost(h models.Host) bool {
	if h.Status != "offline" {
		return false
	}
	diff := h.LastSeen.Sub(h.CreatedAt)
	if diff < 0 {
		diff = -diff
	}
	return diff < neverConnectedToleranceSeconds*time.Second
}

// isTrackerMonitorOnly mirrors useAttentionCenter.ts's isTrackerMonitorOnly:
// a tracker only dispatches an update if it has a task to run — for git
// trackers that's CustomTaskID, for docker trackers it's CustomTaskID
// (default "custom" UpdateAction) or ComposeProject (UpdateAction="compose").
func isTrackerMonitorOnly(t models.ReleaseTracker) bool {
	if t.TrackerType == "git" {
		return t.CustomTaskID == ""
	}
	if t.UpdateAction == "compose" {
		return t.ComposeProject == ""
	}
	return t.CustomTaskID == ""
}

// Attention returns the aggregated "needs attention" feed, item order and
// wording matching frontend/src/composables/useAttentionCenter.ts exactly
// (that file's own doc comment explains why CVE / Proxmox node-health are
// deliberately excluded here: they already have their own backend-driven
// surfaces — GET /apt/cve-summary and the dashboard WS snapshot's
// proxmox_summary — duplicating them into this feed would just be noise).
// Never returns nil; a source that fails to load is skipped rather than
// failing the whole feed, matching the frontend's Promise.allSettled.
func (s *Service) Attention(ctx context.Context) []models.AttentionItem {
	items := []models.AttentionItem{}

	certs, err := s.repo.ListSSLCertificates(ctx)
	if err == nil {
		n := 0
		for _, c := range certs {
			if c.Enabled && c.DaysRemaining != nil && *c.DaysRemaining <= sslExpiryWarningDays {
				n++
			}
		}
		if n > 0 {
			items = append(items, models.AttentionItem{
				Key:      "ssl",
				Label:    fmt.Sprintf("%d certificat%s SSL bientôt expiré%s", n, plural(n), plural(n)),
				Count:    n,
				To:       "/monitoring?tab=ssl",
				Severity: "warning",
			})
		}
	}

	hosts, err := s.repo.GetAllHosts(ctx)
	if err == nil {
		n := 0
		for _, h := range hosts {
			if isNeverConnectedHost(h) {
				n++
			}
		}
		if n > 0 {
			items = append(items, models.AttentionItem{
				Key:      "never-connected",
				Label:    fmt.Sprintf("%d hôte%s enregistré%s sans agent connecté", n, plural(n), plural(n)),
				Count:    n,
				To:       "/",
				Severity: "warning",
			})
		}
	}

	links, err := s.repo.ListProxmoxGuestLinks(ctx, "suggested")
	if err == nil && len(links) > 0 {
		n := len(links)
		items = append(items, models.AttentionItem{
			Key:      "proxmox-links",
			Label:    fmt.Sprintf("%d liaison%s Proxmox suggérée%s à confirmer", n, plural(n), plural(n)),
			Count:    n,
			To:       "/proxmox",
			Severity: "info",
		})
	}

	npmHosts, err := s.repo.ListAllNPMProxyHostsEnriched(ctx)
	if err == nil {
		n := 0
		for _, h := range npmHosts {
			if h.NPMEnabled && !h.MonitoringEnabled {
				n++
			}
		}
		if n > 0 {
			items = append(items, models.AttentionItem{
				Key:      "npm-monitoring",
				Label:    fmt.Sprintf("%d proxy host%s NPM sans monitoring activé", n, plural(n)),
				Count:    n,
				To:       "/npm",
				Severity: "info",
			})
		}
	}

	trackers, err := s.repo.ListReleaseTrackers(ctx)
	if err == nil {
		monitorOnly, drifted := 0, 0
		for _, t := range trackers {
			if !t.Enabled {
				continue
			}
			if isTrackerMonitorOnly(t) {
				monitorOnly++
			}
			if driftDetected, _ := s.repo.TrackerDriftDetected(ctx, t); driftDetected {
				drifted++
			}
		}
		if monitorOnly > 0 {
			items = append(items, models.AttentionItem{
				Key:      "trackers",
				Label:    fmt.Sprintf("%d suivi%s de version sans tâche de déploiement", monitorOnly, plural(monitorOnly)),
				Count:    monitorOnly,
				To:       "/git-webhooks?tab=trackers",
				Severity: "info",
			})
		}
		if drifted > 0 {
			items = append(items, models.AttentionItem{
				Key:      "tracker-drift",
				Label:    fmt.Sprintf("%d conteneur%s en dérive par rapport à la version suivie", drifted, plural(drifted)),
				Count:    drifted,
				To:       "/git-webhooks?tab=trackers",
				Severity: "warning",
			})
		}
	}

	return items
}
