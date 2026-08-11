package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestGetDomainDetails_FilterSortPaginate exercises the interactive-log-drawer
// support added to GetDomainDetails: status/method/path filters (shared across
// the KPI aggregate, top lists and the requests page), sort+dir on the
// requests page, and limit/offset pagination.
func TestGetDomainDetails_FilterSortPaginate(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "web-host"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "web", Hostname: "web.local", IPAddress: "10.0.0.20", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	domain := "test.example.com"
	now := time.Now().UTC()
	report := &models.WebLogReport{
		Source:      "nginx",
		Traffic:     &models.TrafficSummary{},
		Threats:     &models.ThreatSummary{},
		CollectedAt: now,
		Requests: []models.WebRequest{
			{IP: "1.1.1.1", Method: "GET", Path: "/", Status: 200, Bytes: 10, Domain: domain},
			{IP: "1.1.1.2", Method: "GET", Path: "/login", Status: 404, Bytes: 20, Domain: domain},
			{IP: "1.1.1.3", Method: "POST", Path: "/login", Status: 404, Bytes: 30, Domain: domain},
			{IP: "1.1.1.4", Method: "GET", Path: "/dashboard", Status: 500, Bytes: 40, Domain: domain, Blocked: true, BlockedSource: "crowdsec"},
			// InsertWebLogSnapshot runs threatdetect.ClassifyRequests, which flips
			// `suspicious` based on a path/method/UA pattern match, overwriting
			// whatever Category is set here — /wp-login.php matches the WordPress
			// pattern (see threatdetect.Classify), no other row's path/method does.
			{IP: "1.1.1.5", Method: "GET", Path: "/wp-login.php", Status: 403, Bytes: 50, Domain: domain},
		},
	}
	report.TotalRequests = len(report.Requests)
	if err := db.InsertWebLogSnapshot(ctx, hostID, report); err != nil {
		t.Fatalf("insert web log snapshot: %v", err)
	}
	since := now.Add(-time.Hour)

	t.Run("status filter scopes hits and the requests page together", func(t *testing.T) {
		details, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Status: "4xx"}, 50, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails: %v", err)
		}
		if hits := details["hits"].(int64); hits != 3 {
			t.Errorf("expected 3 hits for 4xx (404, 404, 403), got %d", hits)
		}
		reqs := details["requests"].([]map[string]any)
		if len(reqs) != 3 {
			t.Fatalf("expected 3 requests, got %d", len(reqs))
		}
	})

	t.Run("blocked filter", func(t *testing.T) {
		details, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Status: "blocked"}, 50, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails: %v", err)
		}
		reqs := details["requests"].([]map[string]any)
		if len(reqs) != 1 || reqs[0]["ip"] != "1.1.1.4" {
			t.Fatalf("expected exactly the blocked /admin request, got %+v", reqs)
		}
	})

	t.Run("suspicious filter", func(t *testing.T) {
		details, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Status: "suspicious"}, 50, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails: %v", err)
		}
		reqs := details["requests"].([]map[string]any)
		if len(reqs) != 1 || reqs[0]["ip"] != "1.1.1.5" {
			t.Fatalf("expected exactly the suspicious request, got %+v", reqs)
		}
		if suspicious, _ := reqs[0]["suspicious"].(bool); !suspicious {
			t.Errorf("expected the row's own suspicious flag to be true, got %+v", reqs[0])
		}
	})

	t.Run("method + path filters narrow the whole scope, not just the requests page", func(t *testing.T) {
		details, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Method: "POST", Path: "/login"}, 50, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails: %v", err)
		}
		if hits := details["hits"].(int64); hits != 1 {
			t.Errorf("expected 1 hit for POST /login, got %d", hits)
		}
		paths := details["top_paths"].([]map[string]any)
		if len(paths) != 1 || paths[0]["path"] != "/login" {
			t.Errorf("expected top_paths scoped down to just /login, got %+v", paths)
		}
	})

	t.Run("sort by bytes ascending", func(t *testing.T) {
		details, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Sort: "bytes", Dir: "asc"}, 50, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails: %v", err)
		}
		reqs := details["requests"].([]map[string]any)
		if len(reqs) != 5 {
			t.Fatalf("expected all 5 requests, got %d", len(reqs))
		}
		if reqs[0]["bytes"].(int64) != 10 || reqs[len(reqs)-1]["bytes"].(int64) != 50 {
			t.Errorf("expected ascending byte order, got first=%v last=%v", reqs[0]["bytes"], reqs[len(reqs)-1]["bytes"])
		}
	})

	t.Run("pagination via limit/offset, total reflects the filtered count", func(t *testing.T) {
		page1, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Sort: "bytes", Dir: "asc"}, 2, 0)
		if err != nil {
			t.Fatalf("GetDomainDetails page1: %v", err)
		}
		if total := page1["total"].(int64); total != 5 {
			t.Errorf("expected total=5, got %d", total)
		}
		reqs1 := page1["requests"].([]map[string]any)
		if len(reqs1) != 2 || reqs1[0]["bytes"].(int64) != 10 || reqs1[1]["bytes"].(int64) != 20 {
			t.Fatalf("expected first page = [10, 20] bytes, got %+v", reqs1)
		}

		page2, err := db.GetDomainDetails(ctx, domain, since, time.Time{}, "", "", database.DomainDetailsFilter{Sort: "bytes", Dir: "asc"}, 2, 2)
		if err != nil {
			t.Fatalf("GetDomainDetails page2: %v", err)
		}
		reqs2 := page2["requests"].([]map[string]any)
		if len(reqs2) != 2 || reqs2[0]["bytes"].(int64) != 30 || reqs2[1]["bytes"].(int64) != 40 {
			t.Fatalf("expected second page = [30, 40] bytes, got %+v", reqs2)
		}
	})
}
