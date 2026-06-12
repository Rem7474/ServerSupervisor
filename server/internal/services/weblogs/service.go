// Package weblogs is the application/service layer for the web-logs / threats
// views. It owns the CrowdSec ban/unban dispatch, the summary enrichment (top-IP
// geolocation with bounded concurrency + cache, KPI window comparison, threats
// promotion) and the detail reads behind a Repository + Dispatcher port. HTTP
// query parsing/validation stays in the handler.
package weblogs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetWebLogsSummary(ctx context.Context, since time.Time, hostID, source string) (map[string]any, error)
	GetWebLogsTopClientIPs(ctx context.Context, since time.Time, hostID, source string, limit int) ([]map[string]any, error)
	GetWebLogsKPIWindow(ctx context.Context, since, until time.Time, hostID, source string) (map[string]any, error)
	GetIPTimeline(ctx context.Context, ip string, since time.Time, hostID string, limit int) ([]models.WebLogIPTimelineRow, error)
	GetDomainDetails(ctx context.Context, domain string, since time.Time, hostID, source string, limit int) (map[string]any, error)
	GetWebLogsTimeseries(ctx context.Context, since time.Time, hostID, source, bucket string) ([]map[string]any, error)
	GetWebLogsLive(ctx context.Context, hostID, source string, limit int) ([]map[string]any, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the web-logs use-cases.
type Service struct {
	repo       Repository
	dispatcher Dispatcher
}

func NewService(repo Repository, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
}

// BlockIP validates the IP + duration and dispatches a CrowdSec ban, returning
// the queued command id.
func (s *Service) BlockIP(ctx context.Context, hostID, ip, duration, username, clientIP string) (string, error) {
	if _, err := netip.ParseAddr(ip); err != nil {
		return "", apperr.Validation("invalid IP address")
	}
	if _, err := time.ParseDuration(duration); err != nil {
		return "", apperr.Validation("invalid duration (exemples: 1h, 4h, 24h, 168h)")
	}
	r, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "crowdsec",
		Action:      "ban",
		Target:      ip,
		Payload:     fmt.Sprintf(`{"duration":%q}`, duration),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "crowdsec_ban",
			HostID:    hostID,
			IPAddress: clientIP,
			Details:   fmt.Sprintf(`{"ip":%q,"duration":%q}`, ip, duration),
		},
	})
	if err != nil {
		return "", err
	}
	return r.Command.ID, nil
}

// UnblockIP validates the IP and dispatches a CrowdSec unban.
func (s *Service) UnblockIP(ctx context.Context, hostID, ip, username, clientIP string) (string, error) {
	if _, err := netip.ParseAddr(ip); err != nil {
		return "", apperr.Validation("invalid IP address")
	}
	r, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "crowdsec",
		Action:      "unban",
		Target:      ip,
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "crowdsec_unban",
			HostID:    hostID,
			IPAddress: clientIP,
			Details:   fmt.Sprintf(`{"ip":"%s"}`, ip),
		},
	})
	if err != nil {
		return "", err
	}
	return r.Command.ID, nil
}

// SummaryResult is the enriched web-logs summary for a period.
type SummaryResult struct {
	Since   time.Time
	Traffic any
	Threats any
	Compare map[string]any
}

// Summary aggregates the web-logs summary over a period and enriches it with
// top-IP geolocation and a current-vs-previous KPI comparison.
func (s *Service) Summary(ctx context.Context, period time.Duration, hostID, source string) (*SummaryResult, error) {
	since := time.Now().Add(-period)
	summary, err := s.repo.GetWebLogsSummary(ctx, since, hostID, source)
	if err != nil {
		return nil, err
	}

	if traffic, ok := summary["traffic"].(map[string]any); ok {
		if topIPs, err := s.repo.GetWebLogsTopClientIPs(ctx, since, hostID, source, 120); err == nil {
			traffic["top_client_ips"] = topIPs
			traffic["country_distribution"] = countryDistribution(topIPs)
		}
	}

	now := time.Now().UTC()
	currentSince := now.Add(-period)
	previousSince := currentSince.Add(-period)
	currentKPI, err := s.repo.GetWebLogsKPIWindow(ctx, currentSince, now, hostID, source)
	if err != nil {
		return nil, err
	}
	previousKPI, err := s.repo.GetWebLogsKPIWindow(ctx, previousSince, currentSince, hostID, source)
	if err != nil {
		return nil, err
	}
	compare := map[string]any{
		"current":  currentKPI,
		"previous": previousKPI,
		"delta_percent": map[string]any{
			"total_requests": deltaPercent(toFloat(currentKPI["total_requests"]), toFloat(previousKPI["total_requests"])),
			"total_bytes":    deltaPercent(toFloat(currentKPI["total_bytes"]), toFloat(previousKPI["total_bytes"])),
			"ratio_5xx":      deltaPercent(toFloat(currentKPI["ratio_5xx"]), toFloat(previousKPI["ratio_5xx"])),
			"suspicious_ips": deltaPercent(toFloat(currentKPI["suspicious_ips"]), toFloat(previousKPI["suspicious_ips"])),
		},
	}

	threats := summary["threats"]
	promoteBlockedIntoThreats(summary, threats)

	return &SummaryResult{Since: since, Traffic: summary["traffic"], Threats: threats, Compare: compare}, nil
}

// IPTimeline returns the request timeline for an IP.
func (s *Service) IPTimeline(ctx context.Context, ip string, since time.Time, hostID string, limit int) ([]models.WebLogIPTimelineRow, error) {
	return s.repo.GetIPTimeline(ctx, ip, since, hostID, limit)
}

// DomainDetails returns aggregated details for a domain.
func (s *Service) DomainDetails(ctx context.Context, domain string, since time.Time, hostID, source string, limit int) (map[string]any, error) {
	return s.repo.GetDomainDetails(ctx, domain, since, hostID, source, limit)
}

// Timeseries returns bucketed web-log counts.
func (s *Service) Timeseries(ctx context.Context, since time.Time, hostID, source, bucket string) ([]map[string]any, error) {
	return s.repo.GetWebLogsTimeseries(ctx, since, hostID, source, bucket)
}

// Live returns the most recent web-log entries.
func (s *Service) Live(ctx context.Context, hostID, source string, limit int) ([]map[string]any, error) {
	return s.repo.GetWebLogsLive(ctx, hostID, source, limit)
}

// promoteBlockedIntoThreats copies blocked_ips/blocked_requests/blocked_ratio
// from traffic into threats so the BotView can read them without the traffic
// section, and promotes crowdsec_blocked_ips when it exceeds the web-log count.
func promoteBlockedIntoThreats(summary map[string]any, threats any) {
	threatsMap, ok := threats.(map[string]any)
	if !ok {
		return
	}
	if trafficMap, ok := summary["traffic"].(map[string]any); ok {
		for _, k := range []string{"blocked_ips", "blocked_requests", "blocked_ratio"} {
			if v, exists := trafficMap[k]; exists {
				threatsMap[k] = v
			}
		}
	}
	if csBlocked, ok := threatsMap["crowdsec_blocked_ips"].(int64); ok && csBlocked > 0 {
		if csBlocked > anyToInt64(threatsMap["blocked_ips"]) {
			threatsMap["blocked_ips"] = csBlocked
		}
	}
}

// countryDistribution resolves the top IPs to countries and returns a list sorted
// by hits descending.
func countryDistribution(topIPs []map[string]any) []map[string]any {
	countryHits, countryCodes := resolveIPsWithContext(topIPs)
	dist := make([]map[string]any, 0, len(countryHits))
	for country, hits := range countryHits {
		dist = append(dist, map[string]any{
			"country":      country,
			"country_code": countryCodes[country],
			"hits":         hits,
		})
	}
	for i := 0; i < len(dist); i++ {
		for j := i + 1; j < len(dist); j++ {
			if anyToInt64(dist[j]["hits"]) > anyToInt64(dist[i]["hits"]) {
				dist[i], dist[j] = dist[j], dist[i]
			}
		}
	}
	return dist
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	default:
		return 0
	}
}

func deltaPercent(current, previous float64) any {
	if previous == 0 {
		if current == 0 {
			return float64(0)
		}
		return nil
	}
	return ((current - previous) / previous) * 100
}

func anyToInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func isPrivateOrLocalIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return true
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsUnspecified()
}

type ipCountryInfo struct {
	Country     string
	CountryCode string
	UpdatedAt   time.Time
}

var (
	ipCountryCache   = map[string]ipCountryInfo{}
	ipCountryCacheMu sync.RWMutex
)

// resolveIPsWithContext resolves IPs to countries with bounded concurrency (4
// workers), returning aggregated country hits + codes.
func resolveIPsWithContext(topIPs []map[string]any) (map[string]int64, map[string]string) {
	countryHits := make(map[string]int64)
	countryCodes := make(map[string]string)
	var mu sync.Mutex

	const concurrency = 4
	jobs := make(chan map[string]any, concurrency)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				ip := strings.TrimSpace(anyToString(row["ip"]))
				hits := anyToInt64(row["hits"])
				if ip == "" || hits <= 0 {
					continue
				}
				country, code := resolveCountryForIP(ip)
				if country == "" {
					country = "Unknown"
				}
				if code == "" {
					code = "--"
				}
				mu.Lock()
				countryHits[country] += hits
				countryCodes[country] = code
				mu.Unlock()
			}
		}()
	}
	for _, row := range topIPs {
		jobs <- row
	}
	close(jobs)
	wg.Wait()
	return countryHits, countryCodes
}

func resolveCountryForIP(ip string) (string, string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "Unknown", "--"
	}
	if isPrivateOrLocalIP(ip) {
		return "Local / Private", "LAN"
	}

	now := time.Now().UTC()
	ipCountryCacheMu.RLock()
	if cached, ok := ipCountryCache[ip]; ok && now.Sub(cached.UpdatedAt) < 24*time.Hour {
		ipCountryCacheMu.RUnlock()
		return cached.Country, cached.CountryCode
	}
	ipCountryCacheMu.RUnlock()

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://ipwho.is/"+ip+"?fields=success,country,country_code", nil)
	if err != nil {
		return "Unknown", "--"
	}
	resp, err := client.Do(req)
	if err != nil {
		return "Unknown", "--"
	}
	defer func() { _ = resp.Body.Close() }()

	var payload struct {
		Success     bool   `json:"success"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || !payload.Success {
		return "Unknown", "--"
	}

	country := strings.TrimSpace(payload.Country)
	if country == "" {
		country = "Unknown"
	}
	code := strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	if code == "" {
		code = "--"
	}

	ipCountryCacheMu.Lock()
	ipCountryCache[ip] = ipCountryInfo{Country: country, CountryCode: code, UpdatedAt: now}
	ipCountryCacheMu.Unlock()
	return country, code
}
