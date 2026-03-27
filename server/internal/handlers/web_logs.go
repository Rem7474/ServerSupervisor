package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
