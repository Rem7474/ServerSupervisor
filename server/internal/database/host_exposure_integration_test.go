package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestGetHostExposure exercises the real SQL behind the host-exposure
// correlation feature: it must find NPM proxy hosts whose forward_host
// matches a host's IP, then aggregate web_log_requests by the resulting
// domain names — regardless of which host_id actually collected those log
// rows (the reverse-proxy host, not necessarily the backend host being
// queried).
func TestGetHostExposure(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	backendIP := "10.0.0.10"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "backend-host", Name: "backend", Hostname: "backend.local", IPAddress: backendIP, Status: "online",
	}); err != nil {
		t.Fatalf("register backend host: %v", err)
	}
	// The host that actually collects the NPM access logs — deliberately a
	// different host than the backend, mirroring the real topology this
	// feature stitches back together.
	proxyHostID := "npm-host"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: proxyHostID, Name: "npm", Hostname: "npm.local", IPAddress: "10.0.0.1", Status: "online",
	}); err != nil {
		t.Fatalf("register npm host: %v", err)
	}

	conn, err := db.CreateNPMConnection(ctx, models.NPMConnectionRequest{
		Name: "main npm", APIURL: "https://npm.local", Identity: "admin@example.com", Secret: "s3cr3t",
	})
	if err != nil {
		t.Fatalf("create npm connection: %v", err)
	}

	t.Run("no proxy host forwards here means no domains and no query against web logs", func(t *testing.T) {
		exposure, err := db.GetHostExposure(ctx, "10.0.0.99", time.Now().Add(-time.Hour))
		if err != nil {
			t.Fatalf("GetHostExposure: %v", err)
		}
		if len(exposure.Domains) != 0 || exposure.TotalRequests != 0 {
			t.Errorf("expected no domains for an unmatched IP, got %+v", exposure)
		}
	})

	t.Run("empty ip short-circuits without touching npm_proxy_hosts", func(t *testing.T) {
		exposure, err := db.GetHostExposure(ctx, "", time.Now().Add(-time.Hour))
		if err != nil {
			t.Fatalf("GetHostExposure: %v", err)
		}
		if len(exposure.Domains) != 0 {
			t.Errorf("expected empty result for an empty ip, got %+v", exposure)
		}
	})

	if _, err := db.UpsertNPMProxyHost(ctx, models.NPMProxyHost{
		ConnectionID: conn.ID, NPMID: 1, DomainNames: []string{"app.example.com"},
		ForwardHost: backendIP, ForwardPort: 8080, SSLEnabled: true, NPMEnabled: true,
	}); err != nil {
		t.Fatalf("upsert proxy host: %v", err)
	}

	now := time.Now().UTC()
	report := &models.WebLogReport{
		Source:        "npm",
		Traffic:       &models.TrafficSummary{},
		Threats:       &models.ThreatSummary{},
		CollectedAt:   now,
		TotalRequests: 3,
		Requests: []models.WebRequest{
			{IP: "1.2.3.4", Method: "GET", Path: "/", Status: 200, Bytes: 100, Domain: "app.example.com"},
			{IP: "5.6.7.8", Method: "GET", Path: "/admin", Status: 404, Bytes: 50, Domain: "app.example.com", Category: "bot", Blocked: true},
			// Different domain, not routed to the backend host — must not be counted.
			{IP: "9.9.9.9", Method: "GET", Path: "/", Status: 200, Bytes: 10, Domain: "other.example.com"},
		},
	}
	if err := db.InsertWebLogSnapshot(ctx, proxyHostID, report); err != nil {
		t.Fatalf("insert web log snapshot: %v", err)
	}

	t.Run("aggregates only the matched domain's requests", func(t *testing.T) {
		exposure, err := db.GetHostExposure(ctx, backendIP, now.Add(-time.Hour))
		if err != nil {
			t.Fatalf("GetHostExposure: %v", err)
		}
		if exposure.IPAddress != backendIP {
			t.Errorf("expected IPAddress %q, got %q", backendIP, exposure.IPAddress)
		}
		if len(exposure.Domains) != 1 {
			t.Fatalf("expected exactly one matching proxy host, got %d", len(exposure.Domains))
		}
		d := exposure.Domains[0]
		if len(d.DomainNames) != 1 || d.DomainNames[0] != "app.example.com" {
			t.Errorf("expected domain_names=[app.example.com], got %v", d.DomainNames)
		}
		if d.Requests != 2 {
			t.Errorf("expected 2 requests for app.example.com, got %d", d.Requests)
		}
		if d.Bytes != 150 {
			t.Errorf("expected 150 bytes, got %d", d.Bytes)
		}
		if d.Errors4xx != 1 {
			t.Errorf("expected 1 4xx error, got %d", d.Errors4xx)
		}
		if d.SuspiciousRequests != 1 {
			t.Errorf("expected 1 suspicious request, got %d", d.SuspiciousRequests)
		}
		if d.BlockedRequests != 1 {
			t.Errorf("expected 1 blocked request, got %d", d.BlockedRequests)
		}
		if exposure.TotalRequests != 2 || exposure.TotalSuspicious != 1 || exposure.TotalBlocked != 1 {
			t.Errorf("unexpected totals: %+v", exposure)
		}
	})

	t.Run("requests outside the window are excluded", func(t *testing.T) {
		exposure, err := db.GetHostExposure(ctx, backendIP, now.Add(time.Hour))
		if err != nil {
			t.Fatalf("GetHostExposure: %v", err)
		}
		if exposure.Domains[0].Requests != 0 {
			t.Errorf("expected 0 requests when since is after all captured_at, got %d", exposure.Domains[0].Requests)
		}
	})
}
