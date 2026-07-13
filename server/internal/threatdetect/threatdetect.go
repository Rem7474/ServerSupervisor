// Package threatdetect classifies web-log requests into attack-pattern
// categories and scores suspicious IPs from the aggregated result. This used
// to be decided by the agent (agent/internal/collector/web_logs.go); it now
// lives here so an admin can retune detection for the whole fleet from the
// Settings UI without redeploying agents, and so the server has direct
// access to the response status code when scoring — the agent's local
// per-report view couldn't weigh "many hits that all 404'd" differently from
// "many hits that all succeeded".
package threatdetect

import (
	"math"
	"strings"

	"github.com/serversupervisor/server/internal/models"
)

// Category names — also the literal values stored in the
// `web_log_requests.category` column.
const (
	CategoryWordPress        = "WordPress"
	CategoryAdminPanel       = "AdminPanel"
	CategoryPathTraversal    = "PathTraversal"
	CategoryKnownScanner     = "KnownScanner"
	CategorySuspiciousMethod = "SuspiciousMethod"
)

var pathNeedles = []string{
	"/.env", "/wp-admin", "/wp-login", "/xmlrpc.php", "/cgi-bin", "/phpmyadmin", "/pma",
	"/manager/html", "/actuator", "/.git", "/vendor/phpunit", "/solr", "/hudson", "/jenkins",
	"/autodiscover", "/owa", "../", "/etc/passwd", "/bin/bash", "/struts", "/boaform", "/api/jsonws",
}

var uaNeedles = []string{
	"masscan", "nmap", "zgrab", "sqlmap", "nikto", "dirbuster", "gobuster", "wpscan", "acunetix", "nessus",
}

// Classify returns the attack-pattern category matched by this request, or
// "" when it looks benign. The pattern list is still fixed in code — only
// the per-category/per-status weights used to turn matches into a score are
// admin-configurable (see Weights).
func Classify(method, path, userAgent string) string {
	pathLower := strings.ToLower(path)
	switch {
	case strings.Contains(pathLower, "/wp-") || strings.Contains(pathLower, "/xmlrpc.php"):
		return CategoryWordPress
	case strings.Contains(pathLower, "/admin") || strings.Contains(pathLower, "/manager/html") || strings.Contains(pathLower, "/phpmyadmin"):
		return CategoryAdminPanel
	case strings.Contains(pathLower, "../") || strings.Contains(pathLower, "/etc/passwd") || strings.Contains(pathLower, "/bin/bash"):
		return CategoryPathTraversal
	}

	for _, needle := range pathNeedles {
		if strings.Contains(pathLower, needle) {
			return CategoryKnownScanner
		}
	}

	uaLower := strings.ToLower(userAgent)
	for _, needle := range uaNeedles {
		if strings.Contains(uaLower, needle) {
			return CategoryKnownScanner
		}
	}

	switch strings.ToUpper(method) {
	case "OPTIONS", "PROPFIND", "TRACE", "CONNECT":
		return CategorySuspiciousMethod
	}

	return ""
}

// ClassifyRequests fills in Category for every request in place. The agent
// no longer decides this (see agent/CLAUDE.md's protocol contract note) —
// call this once, server-side, right after decoding an agent report and
// before persisting it.
func ClassifyRequests(requests []models.WebRequest) {
	for i := range requests {
		requests[i].Category = Classify(requests[i].Method, requests[i].Path, requests[i].UserAgent)
	}
}

// SummarizeSuspicious returns how many requests are suspicious (Category
// already populated by ClassifyRequests) and how many distinct IPs they came
// from — the small per-report counters stored on the hosts cache and on the
// web_log_snapshots row.
func SummarizeSuspicious(requests []models.WebRequest) (count int64, uniqueIPs int64) {
	ips := make(map[string]struct{})
	for _, r := range requests {
		if r.Category == "" {
			continue
		}
		count++
		ips[r.IP] = struct{}{}
	}
	return count, int64(len(ips))
}

// CategoryCounts is how many of an IP's suspicious hits fell into each
// category, used to weight its score by attack severity.
type CategoryCounts struct {
	WordPress        int64
	AdminPanel       int64
	PathTraversal    int64
	KnownScanner     int64
	SuspiciousMethod int64
}

// StatusCounts buckets an IP's suspicious hits by HTTP response status, used
// to weight its score by how the target actually responded — many hits that
// all succeeded (2xx) are far less suspicious than a handful that broke
// something (5xx) or probed for resources that don't exist (404).
type StatusCounts struct {
	Status2xx      int64
	Status3xx      int64
	Status404      int64
	Status4xxOther int64
	Status5xx      int64
}

// Weights are the admin-tunable coefficients behind the threat score.
// Always start from DefaultWeights() and override — the zero value scores
// everything at 0 (LOW).
type Weights struct {
	CategoryWordPress        float64
	CategoryAdminPanel       float64
	CategoryPathTraversal    float64
	CategoryKnownScanner     float64
	CategorySuspiciousMethod float64

	Status2xxMultiplier      float64
	Status3xxMultiplier      float64
	Status404Multiplier      float64
	Status4xxOtherMultiplier float64
	Status5xxMultiplier      float64

	BreadthWeight float64 // per unique path targeted
	HitsWeight    float64 // scales the (dampened) hit-volume term

	// Score cutoffs for MEDIUM/HIGH/CRITICAL (below ThresholdMedium is LOW).
	// Keep these ascending — Score() checks CRITICAL first, so a
	// misconfigured ThresholdCritical lower than ThresholdMedium would make
	// everything above it read CRITICAL instead of MEDIUM.
	ThresholdMedium   float64
	ThresholdHigh     float64
	ThresholdCritical float64
}

// DefaultWeights returns the out-of-the-box calibration. Tuned so that (a) a
// single endpoint hammered many times but always returning 2xx stays LOW
// regardless of hit count, and (b) a handful of hits that mostly 404/5xx —
// or that spread across many distinct paths — reach MEDIUM/HIGH quickly even
// at low volume. That inversion (status/breadth matter more than raw hit
// count) is what this package exists to fix vs. the old hits×unique_paths
// formula.
func DefaultWeights() Weights {
	return Weights{
		CategoryWordPress:        2,
		CategoryAdminPanel:       3,
		CategoryPathTraversal:    5,
		CategoryKnownScanner:     4,
		CategorySuspiciousMethod: 2,

		Status2xxMultiplier:      0.1,
		Status3xxMultiplier:      1,
		Status404Multiplier:      2,
		Status4xxOtherMultiplier: 1.5,
		Status5xxMultiplier:      3,

		BreadthWeight: 3,
		HitsWeight:    2,

		ThresholdMedium:   15,
		ThresholdHigh:     50,
		ThresholdCritical: 150,
	}
}

// Score computes a threat score for one IP from its suspicious-hit category
// and status breakdown, plus the LOW/MEDIUM/HIGH/CRITICAL level it maps to.
//
//	score = uniquePaths×BreadthWeight×avgCategoryWeight
//	      + ln(hits+1)×HitsWeight×avgCategoryWeight×avgStatusMultiplier
//
// avgCategoryWeight/avgStatusMultiplier are hit-weighted averages over the
// category/status mix, so a flood of hits that are overwhelmingly 2xx (a
// legitimate-looking, successful pattern) is heavily discounted, while a
// small number of hits that are mostly 404/5xx — or that fan out across many
// distinct paths — climb the score quickly despite the low volume. hits
// enters through ln(hits+1) rather than linearly so raw request volume alone
// can no longer dominate the way it did in the old hits×unique_paths score.
func Score(hits, uniquePaths int64, cat CategoryCounts, st StatusCounts, w Weights) (float64, string) {
	if hits <= 0 {
		return 0, "LOW"
	}
	h := float64(hits)
	avgCategory := (float64(cat.WordPress)*w.CategoryWordPress +
		float64(cat.AdminPanel)*w.CategoryAdminPanel +
		float64(cat.PathTraversal)*w.CategoryPathTraversal +
		float64(cat.KnownScanner)*w.CategoryKnownScanner +
		float64(cat.SuspiciousMethod)*w.CategorySuspiciousMethod) / h
	avgStatus := (float64(st.Status2xx)*w.Status2xxMultiplier +
		float64(st.Status3xx)*w.Status3xxMultiplier +
		float64(st.Status404)*w.Status404Multiplier +
		float64(st.Status4xxOther)*w.Status4xxOtherMultiplier +
		float64(st.Status5xx)*w.Status5xxMultiplier) / h

	score := float64(uniquePaths)*w.BreadthWeight*avgCategory +
		math.Log(h+1)*w.HitsWeight*avgCategory*avgStatus

	level := "LOW"
	switch {
	case score >= w.ThresholdCritical:
		level = "CRITICAL"
	case score >= w.ThresholdHigh:
		level = "HIGH"
	case score >= w.ThresholdMedium:
		level = "MEDIUM"
	}
	return score, level
}
