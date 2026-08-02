package dashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	hosts       []models.Host
	hostsErr    error
	links       []models.ProxmoxGuestLink
	linksErr    error
	npmHosts    []models.NPMProxyHostEnriched
	npmErr      error
	trackers    []models.ReleaseTracker
	trackersErr error
	drift       map[string]bool
	certs       []models.SSLCertificate
	certsErr    error
}

func (f *fakeRepo) GetAllHosts(context.Context) ([]models.Host, error) { return f.hosts, f.hostsErr }
func (f *fakeRepo) ListProxmoxGuestLinks(context.Context, string) ([]models.ProxmoxGuestLink, error) {
	return f.links, f.linksErr
}
func (f *fakeRepo) ListAllNPMProxyHostsEnriched(context.Context) ([]models.NPMProxyHostEnriched, error) {
	return f.npmHosts, f.npmErr
}
func (f *fakeRepo) ListReleaseTrackers(context.Context) ([]models.ReleaseTracker, error) {
	return f.trackers, f.trackersErr
}
func (f *fakeRepo) TrackerDriftDetected(_ context.Context, t models.ReleaseTracker) (bool, error) {
	return f.drift[t.ID], nil
}
func (f *fakeRepo) ListSSLCertificates(context.Context) ([]models.SSLCertificate, error) {
	return f.certs, f.certsErr
}

func intPtr(n int) *int { return &n }

func TestAttention_Empty(t *testing.T) {
	svc := NewService(&fakeRepo{})
	items := svc.Attention(context.Background())
	if items == nil {
		t.Fatal("Attention() must never return nil")
	}
	if len(items) != 0 {
		t.Errorf("expected no items for an empty repo, got %d: %+v", len(items), items)
	}
}

func TestAttention_OrderAndContent(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{
		certs: []models.SSLCertificate{
			{Enabled: true, DaysRemaining: intPtr(3)},
			{Enabled: true, DaysRemaining: intPtr(20)},   // not expiring soon — excluded
			{Enabled: false, DaysRemaining: intPtr(1)},   // disabled — excluded
			{Enabled: true, DaysRemaining: nil},           // never checked yet — excluded
		},
		hosts: []models.Host{
			{Status: "offline", CreatedAt: now, LastSeen: now}, // never connected
			{Status: "online", CreatedAt: now.Add(-time.Hour), LastSeen: now},
		},
		links: []models.ProxmoxGuestLink{{}, {}},
		npmHosts: []models.NPMProxyHostEnriched{
			{NPMProxyHost: models.NPMProxyHost{NPMEnabled: true, MonitoringEnabled: false}},
			{NPMProxyHost: models.NPMProxyHost{NPMEnabled: true, MonitoringEnabled: true}}, // monitoring on — excluded
			{NPMProxyHost: models.NPMProxyHost{NPMEnabled: false, MonitoringEnabled: false}}, // npm off — excluded
		},
		trackers: []models.ReleaseTracker{
			{ID: "t1", Enabled: true, TrackerType: "git", CustomTaskID: ""},                                   // monitor-only
			{ID: "t2", Enabled: true, TrackerType: "docker", UpdateAction: "compose", ComposeProject: "web"},   // has a task, and will drift
			{ID: "t3", Enabled: false, TrackerType: "git", CustomTaskID: ""},                                   // disabled — excluded entirely
		},
		drift: map[string]bool{"t2": true},
	}

	items := NewService(repo).Attention(context.Background())

	wantKeys := []string{"ssl", "never-connected", "proxmox-links", "npm-monitoring", "trackers", "tracker-drift"}
	if len(items) != len(wantKeys) {
		t.Fatalf("expected %d items, got %d: %+v", len(wantKeys), len(items), items)
	}
	for i, want := range wantKeys {
		if items[i].Key != want {
			t.Errorf("item %d: expected key %q, got %q", i, want, items[i].Key)
		}
	}

	byKey := map[string]models.AttentionItem{}
	for _, it := range items {
		byKey[it.Key] = it
	}

	ssl := byKey["ssl"]
	if ssl.Count != 1 || ssl.Label != "1 certificat SSL bientôt expiré" || ssl.To != "/monitoring?tab=ssl" || ssl.Severity != "warning" {
		t.Errorf("ssl item mismatch: %+v", ssl)
	}

	neverConnected := byKey["never-connected"]
	if neverConnected.Count != 1 || neverConnected.Label != "1 hôte enregistré sans agent connecté" || neverConnected.To != "/" || neverConnected.Severity != "warning" {
		t.Errorf("never-connected item mismatch: %+v", neverConnected)
	}

	proxmoxLinks := byKey["proxmox-links"]
	if proxmoxLinks.Count != 2 || proxmoxLinks.Label != "2 liaisons Proxmox suggérées à confirmer" || proxmoxLinks.To != "/proxmox" || proxmoxLinks.Severity != "info" {
		t.Errorf("proxmox-links item mismatch: %+v", proxmoxLinks)
	}

	npmMonitoring := byKey["npm-monitoring"]
	if npmMonitoring.Count != 1 || npmMonitoring.Label != "1 proxy host NPM sans monitoring activé" || npmMonitoring.To != "/npm" || npmMonitoring.Severity != "info" {
		t.Errorf("npm-monitoring item mismatch: %+v", npmMonitoring)
	}

	trackers := byKey["trackers"]
	if trackers.Count != 1 || trackers.Label != "1 suivi de version sans tâche de déploiement" || trackers.To != "/git-webhooks?tab=trackers" || trackers.Severity != "info" {
		t.Errorf("trackers item mismatch: %+v", trackers)
	}

	drift := byKey["tracker-drift"]
	if drift.Count != 1 || drift.Label != "1 conteneur en dérive par rapport à la version suivie" || drift.To != "/git-webhooks?tab=trackers" || drift.Severity != "warning" {
		t.Errorf("tracker-drift item mismatch: %+v", drift)
	}
}

func TestAttention_PluralForms(t *testing.T) {
	repo := &fakeRepo{
		certs: []models.SSLCertificate{
			{Enabled: true, DaysRemaining: intPtr(1)},
			{Enabled: true, DaysRemaining: intPtr(2)},
		},
	}
	items := NewService(repo).Attention(context.Background())
	if len(items) != 1 {
		t.Fatalf("expected exactly the ssl item, got %+v", items)
	}
	if items[0].Label != "2 certificats SSL bientôt expirés" {
		t.Errorf("plural form mismatch, got %q", items[0].Label)
	}
}

func TestAttention_SkipsFailedSource(t *testing.T) {
	repo := &fakeRepo{
		hostsErr: errors.New("db unreachable"),
		certs: []models.SSLCertificate{
			{Enabled: true, DaysRemaining: intPtr(1)},
		},
	}
	items := NewService(repo).Attention(context.Background())
	if len(items) != 1 || items[0].Key != "ssl" {
		t.Errorf("a failed source should be skipped, not fail the whole feed: got %+v", items)
	}
}

func TestIsNeverConnectedHost(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		host models.Host
		want bool
	}{
		{"offline, last_seen == created_at", models.Host{Status: "offline", CreatedAt: now, LastSeen: now}, true},
		{"offline, last_seen 30s after created_at", models.Host{Status: "offline", CreatedAt: now, LastSeen: now.Add(30 * time.Second)}, true},
		{"offline, last_seen far after created_at", models.Host{Status: "offline", CreatedAt: now, LastSeen: now.Add(time.Hour)}, false},
		{"online, timestamps close", models.Host{Status: "online", CreatedAt: now, LastSeen: now}, false},
		{"warning status, timestamps close", models.Host{Status: "warning", CreatedAt: now, LastSeen: now}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNeverConnectedHost(tc.host); got != tc.want {
				t.Errorf("isNeverConnectedHost() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsTrackerMonitorOnly(t *testing.T) {
	cases := []struct {
		name    string
		tracker models.ReleaseTracker
		want    bool
	}{
		{"git without custom task", models.ReleaseTracker{TrackerType: "git"}, true},
		{"git with custom task", models.ReleaseTracker{TrackerType: "git", CustomTaskID: "task-1"}, false},
		{"compose without compose project", models.ReleaseTracker{TrackerType: "docker", UpdateAction: "compose"}, true},
		{"compose with compose project", models.ReleaseTracker{TrackerType: "docker", UpdateAction: "compose", ComposeProject: "web"}, false},
		{"custom docker without custom task", models.ReleaseTracker{TrackerType: "docker", UpdateAction: "custom"}, true},
		{"custom docker with custom task", models.ReleaseTracker{TrackerType: "docker", UpdateAction: "custom", CustomTaskID: "task-1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTrackerMonitorOnly(tc.tracker); got != tc.want {
				t.Errorf("isTrackerMonitorOnly() = %v, want %v", got, tc.want)
			}
		})
	}
}
