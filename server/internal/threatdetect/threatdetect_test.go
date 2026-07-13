package threatdetect

import (
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		ua     string
		want   string
	}{
		{"wordpress path", "GET", "/wp-login.php", "Mozilla/5.0", CategoryWordPress},
		{"xmlrpc", "POST", "/xmlrpc.php", "Mozilla/5.0", CategoryWordPress},
		{"admin panel", "GET", "/admin/login", "Mozilla/5.0", CategoryAdminPanel},
		{"phpmyadmin", "GET", "/phpmyadmin/index.php", "Mozilla/5.0", CategoryAdminPanel},
		{"path traversal", "GET", "/../../etc/passwd", "Mozilla/5.0", CategoryPathTraversal},
		{"known scanner path", "GET", "/.env", "Mozilla/5.0", CategoryKnownScanner},
		{"known scanner ua", "GET", "/", "sqlmap/1.6", CategoryKnownScanner},
		{"suspicious method", "PROPFIND", "/", "Mozilla/5.0", CategorySuspiciousMethod},
		{"benign", "GET", "/", "Mozilla/5.0 (real browser)", ""},
		{"benign api call", "POST", "/api/v1/orders", "MyApp/1.0", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.method, tc.path, tc.ua); got != tc.want {
				t.Errorf("Classify(%q, %q, %q) = %q, want %q", tc.method, tc.path, tc.ua, got, tc.want)
			}
		})
	}
}

func TestClassifyRequests(t *testing.T) {
	requests := []models.WebRequest{
		{Method: "GET", Path: "/wp-admin", UserAgent: "curl"},
		{Method: "GET", Path: "/", UserAgent: "Mozilla/5.0"},
	}
	ClassifyRequests(requests)
	if requests[0].Category != CategoryWordPress {
		t.Errorf("requests[0].Category = %q, want %q", requests[0].Category, CategoryWordPress)
	}
	if requests[1].Category != "" {
		t.Errorf("requests[1].Category = %q, want empty", requests[1].Category)
	}
}

func TestSummarizeSuspicious(t *testing.T) {
	requests := []models.WebRequest{
		{IP: "1.1.1.1", Category: CategoryWordPress},
		{IP: "1.1.1.1", Category: CategoryWordPress},
		{IP: "2.2.2.2", Category: CategoryKnownScanner},
		{IP: "3.3.3.3", Category: ""},
	}
	count, uniqueIPs := SummarizeSuspicious(requests)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
	if uniqueIPs != 2 {
		t.Errorf("uniqueIPs = %d, want 2", uniqueIPs)
	}
}

// TestScore_HighVolumeAllSuccessStaysLow guards the exact regression this
// package was written to fix: a single endpoint hammered many times but
// always returning 2xx must not read as CRITICAL just because hits is huge.
func TestScore_HighVolumeAllSuccessStaysLow(t *testing.T) {
	w := DefaultWeights()
	cat := CategoryCounts{WordPress: 5000}
	st := StatusCounts{Status2xx: 5000}
	score, level := Score(5000, 1, cat, st, w)
	if level != "LOW" {
		t.Errorf("5000 hits, 1 path, all 2xx: level = %s (score %.2f), want LOW", level, score)
	}
}

// TestScore_LowVolumeErrorHeavyElevates guards the other half: a handful of
// hits that mostly fail (404/5xx) must climb the score fast, not stay LOW
// just because the raw hit count is small.
func TestScore_LowVolumeErrorHeavyElevates(t *testing.T) {
	w := DefaultWeights()
	cat := CategoryCounts{PathTraversal: 3}
	st := StatusCounts{Status5xx: 3}
	score, level := Score(3, 1, cat, st, w)
	if level == "LOW" {
		t.Errorf("3 hits, 1 path, all 5xx PathTraversal: level = LOW (score %.2f), want at least MEDIUM", score)
	}
}

func TestScore_BroadScanIsMostSevere(t *testing.T) {
	w := DefaultWeights()
	cat := CategoryCounts{KnownScanner: 20}
	st := StatusCounts{Status404: 15, Status2xx: 5}
	score, level := Score(20, 15, cat, st, w)
	if level != "CRITICAL" {
		t.Errorf("20 hits across 15 distinct paths, KnownScanner, mostly 404: level = %s (score %.2f), want CRITICAL", level, score)
	}
}

func TestScore_ZeroHits(t *testing.T) {
	score, level := Score(0, 0, CategoryCounts{}, StatusCounts{}, DefaultWeights())
	if score != 0 || level != "LOW" {
		t.Errorf("Score(0, ...) = (%.2f, %s), want (0, LOW)", score, level)
	}
}
