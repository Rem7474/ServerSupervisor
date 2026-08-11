package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestGetWebLogsSummary_ParallelFanOutMatchesExpectedAggregates seeds a small,
// hand-computable batch of requests and asserts the aggregate counts survive
// GetWebLogsSummary's concurrent fan-out (traffic aggregates, threats and
// CrowdSec sections now run in parallel goroutines writing into separate
// maps that get merged afterward) without any section being dropped,
// duplicated, or raced. Run under `go test -race`, this also exercises the
// concurrent map writes for data races.
func TestGetWebLogsSummary_ParallelFanOutMatchesExpectedAggregates(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "web-logs-summary-host"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "edge", Hostname: "edge.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	now := time.Now().UTC()
	report := &models.WebLogReport{
		Source:      "npm",
		Traffic:     &models.TrafficSummary{},
		Threats:     &models.ThreatSummary{},
		CollectedAt: now,
		Requests: []models.WebRequest{
			{IP: "1.1.1.1", Method: "GET", Path: "/", Status: 200, Bytes: 100, Domain: "app.example.com"},
			{IP: "1.1.1.2", Method: "GET", Path: "/", Status: 200, Bytes: 200, Domain: "app.example.com"},
			{IP: "1.1.1.3", Method: "GET", Path: "/wp-admin", Status: 404, Bytes: 50, Domain: "app.example.com"},
			{IP: "1.1.1.4", Method: "GET", Path: "/", Status: 500, Bytes: 10, Domain: "app.example.com"},
			{IP: "1.1.1.5", Method: "GET", Path: "/", Status: 403, Bytes: 5, Domain: "app.example.com", Blocked: true},
			{IP: "2.2.2.1", Method: "GET", Path: "/", Status: 200, Bytes: 300, Domain: "other.example.com"},
			{IP: "2.2.2.2", Method: "GET", Path: "/", Status: 200, Bytes: 300, Domain: "other.example.com"},
		},
	}
	if err := db.InsertWebLogSnapshot(ctx, hostID, report); err != nil {
		t.Fatalf("insert web log snapshot: %v", err)
	}

	since := now.Add(-time.Hour)

	t.Run("GetWebLogsSummary aggregates match the seeded requests", func(t *testing.T) {
		result, err := db.GetWebLogsSummary(ctx, since, time.Time{}, "", "")
		if err != nil {
			t.Fatalf("GetWebLogsSummary: %v", err)
		}
		traffic, ok := result["traffic"].(map[string]any)
		if !ok {
			t.Fatalf("expected a traffic map in the result, got %+v", result)
		}
		threats, ok := result["threats"].(map[string]any)
		if !ok {
			t.Fatalf("expected a threats map in the result, got %+v", result)
		}

		if got := traffic["total_requests"].(int64); got != 7 {
			t.Errorf("expected total_requests=7, got %d", got)
		}
		if got := traffic["total_bytes"].(int64); got != 965 {
			t.Errorf("expected total_bytes=965, got %d", got)
		}
		if got := traffic["errors_4xx"].(int64); got != 2 {
			t.Errorf("expected errors_4xx=2 (404+403), got %d", got)
		}
		if got := traffic["errors_5xx"].(int64); got != 1 {
			t.Errorf("expected errors_5xx=1, got %d", got)
		}
		if got := traffic["blocked_requests"].(int64); got != 1 {
			t.Errorf("expected blocked_requests=1, got %d", got)
		}

		topDomains, ok := traffic["top_domains"].([]map[string]any)
		if !ok || len(topDomains) != 2 {
			t.Fatalf("expected 2 top_domains entries, got %+v", traffic["top_domains"])
		}
		var appDomain map[string]any
		for _, d := range topDomains {
			if d["domain"] == "app.example.com" {
				appDomain = d
			}
		}
		if appDomain == nil {
			t.Fatal("expected an app.example.com entry in top_domains")
		}
		if got := appDomain["hits"].(int64); got != 5 {
			t.Errorf("expected app.example.com hits=5, got %d", got)
		}

		if got := threats["suspicious_requests"].(int64); got != 1 {
			t.Errorf("expected suspicious_requests=1, got %d", got)
		}
		if got := threats["suspicious_ips"].(int64); got != 1 {
			t.Errorf("expected suspicious_ips=1, got %d", got)
		}
	})

	t.Run("GetWebLogsThreats (threats-only scope) matches the same suspicious/blocked counts", func(t *testing.T) {
		threats, err := db.GetWebLogsThreats(ctx, since, time.Time{}, "", "")
		if err != nil {
			t.Fatalf("GetWebLogsThreats: %v", err)
		}
		if got := threats["suspicious_requests"].(int64); got != 1 {
			t.Errorf("expected suspicious_requests=1, got %d", got)
		}
		if got := threats["blocked_ips"].(int64); got != 1 {
			t.Errorf("expected blocked_ips=1, got %d", got)
		}
	})
}

// TestGetWebLogsSummary_UntilExcludesRequestsPastTheUpperBound guards the
// custom-range support added to buildWebLogsWhere: a non-zero `until` must
// exclude requests captured after it, not just requests before `since` —
// the pre-existing behavior (until always zero) only ever tested the lower
// bound.
func TestGetWebLogsSummary_UntilExcludesRequestsPastTheUpperBound(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "web-logs-until-host"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "edge", Hostname: "edge.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	now := time.Now().UTC()
	since := now.Add(-3 * time.Hour)
	until := now.Add(-1 * time.Hour)

	report := &models.WebLogReport{
		Source:      "npm",
		Traffic:     &models.TrafficSummary{},
		Threats:     &models.ThreatSummary{},
		CollectedAt: now,
		Requests: []models.WebRequest{
			// Before the window: must be excluded by since.
			{Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339), IP: "1.1.1.1", Method: "GET", Path: "/", Status: 200, Bytes: 1, Domain: "app.example.com"},
			// Inside [since, until): must be counted.
			{Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), IP: "1.1.1.2", Method: "GET", Path: "/", Status: 200, Bytes: 1, Domain: "app.example.com"},
			// After the window: must be excluded by until.
			{Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339), IP: "1.1.1.3", Method: "GET", Path: "/", Status: 200, Bytes: 1, Domain: "app.example.com"},
		},
	}
	if err := db.InsertWebLogSnapshot(ctx, hostID, report); err != nil {
		t.Fatalf("insert web log snapshot: %v", err)
	}

	result, err := db.GetWebLogsSummary(ctx, since, until, "", "")
	if err != nil {
		t.Fatalf("GetWebLogsSummary: %v", err)
	}
	traffic := result["traffic"].(map[string]any)
	if got := traffic["total_requests"].(int64); got != 1 {
		t.Errorf("expected exactly the 1 in-window request, got total_requests=%d", got)
	}

	// Sanity check: the same query with until=zero (open ended) must count
	// both the in-window and the after-window request, confirming the
	// exclusion above is actually caused by until, not some other filter.
	openEnded, err := db.GetWebLogsSummary(ctx, since, time.Time{}, "", "")
	if err != nil {
		t.Fatalf("GetWebLogsSummary (open-ended): %v", err)
	}
	openTraffic := openEnded["traffic"].(map[string]any)
	if got := openTraffic["total_requests"].(int64); got != 2 {
		t.Errorf("expected 2 requests with an open-ended until, got total_requests=%d", got)
	}
}
