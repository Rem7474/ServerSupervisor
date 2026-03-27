package handlers

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ipCountryInfo struct {
	Country     string
	CountryCode string
	UpdatedAt   time.Time
}

var (
	ipCountryCache   = map[string]ipCountryInfo{}
	ipCountryCacheMu sync.RWMutex
)

func (h *AuthHandler) GetWebLogsSummary(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	periodRaw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(periodRaw)
	if err != nil || period <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period (example: 24h, 168h)"})
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if source != "" {
		switch source {
		case "npm", "nginx", "apache", "caddy":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source"})
			return
		}
	}

	since := time.Now().Add(-period)
	summary, err := h.db.GetWebLogsSummary(since, hostID, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to aggregate web logs"})
		return
	}

	if traffic, ok := summary["traffic"].(map[string]any); ok {
		topIPs, err := h.db.GetWebLogsTopClientIPs(since, hostID, source, 120)
		if err == nil {
			traffic["top_client_ips"] = topIPs

			countryHits := map[string]int64{}
			countryCodes := map[string]string{}
			for _, row := range topIPs {
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
				countryHits[country] += hits
				countryCodes[country] = code
			}

			dist := make([]map[string]any, 0, len(countryHits))
			for country, hits := range countryHits {
				dist = append(dist, map[string]any{
					"country":      country,
					"country_code": countryCodes[country],
					"hits":         hits,
				})
			}
			// Sort descending by hits.
			for i := 0; i < len(dist); i++ {
				for j := i + 1; j < len(dist); j++ {
					if anyToInt64(dist[j]["hits"]) > anyToInt64(dist[i]["hits"]) {
						dist[i], dist[j] = dist[j], dist[i]
					}
				}
			}
			traffic["country_distribution"] = dist
		}
	}

	now := time.Now().UTC()
	currentSince := now.Add(-period)
	previousSince := currentSince.Add(-period)
	currentKPI, err := h.db.GetWebLogsKPIWindow(currentSince, now, hostID, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute KPI comparison"})
		return
	}
	previousKPI, err := h.db.GetWebLogsKPIWindow(previousSince, currentSince, hostID, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute KPI comparison"})
		return
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

	c.JSON(http.StatusOK, gin.H{
		"period":  periodRaw,
		"since":   since,
		"host_id": hostID,
		"source":  source,
		"traffic": summary["traffic"],
		"threats": summary["threats"],
		"compare": compare,
	})
}

func (h *AuthHandler) GetWebLogsIPTimeline(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip is required"})
		return
	}

	hostID := strings.TrimSpace(c.Query("host_id"))
	periodRaw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(periodRaw)
	if err != nil || period <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period (example: 24h, 168h)"})
		return
	}

	limit := 500
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	since := time.Now().Add(-period)
	rows, err := h.db.GetIPTimeline(ip, since, hostID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load IP timeline"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ip":       ip,
		"host_id":  hostID,
		"period":   periodRaw,
		"since":    since,
		"count":    len(rows),
		"requests": rows,
	})
}

func (h *AuthHandler) GetWebLogsDomainDetails(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	domain := strings.TrimSpace(c.Param("domain"))
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain is required"})
		return
	}

	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	periodRaw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(periodRaw)
	if err != nil || period <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period (example: 24h, 168h)"})
		return
	}

	limit := 300
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	since := time.Now().Add(-period)
	data, err := h.db.GetDomainDetails(domain, since, hostID, source, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load domain details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"domain":  domain,
		"period":  periodRaw,
		"since":   since,
		"host_id": hostID,
		"source":  source,
		"details": data,
	})
}

func (h *AuthHandler) GetWebLogsTimeseries(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	periodRaw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(periodRaw)
	if err != nil || period <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid period (example: 24h, 168h)"})
		return
	}

	bucket := strings.ToLower(strings.TrimSpace(c.DefaultQuery("bucket", "hour")))
	switch bucket {
	case "hour", "minute":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bucket (hour|minute)"})
		return
	}

	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if source != "" {
		switch source {
		case "npm", "nginx", "apache", "caddy":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source"})
			return
		}
	}

	since := time.Now().Add(-period)
	points, err := h.db.GetWebLogsTimeseries(since, hostID, source, bucket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load web logs timeseries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"period":  periodRaw,
		"bucket":  bucket,
		"since":   since,
		"host_id": hostID,
		"source":  source,
		"points":  points,
	})
}

func (h *AuthHandler) GetWebLogsLive(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if source != "" {
		switch source {
		case "npm", "nginx", "apache", "caddy":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source"})
			return
		}
	}

	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	rows, err := h.db.GetWebLogsLive(hostID, source, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load web logs live feed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"host_id":  hostID,
		"source":   source,
		"count":    len(rows),
		"requests": rows,
	})
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

func deltaPercent(current float64, previous float64) any {
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
