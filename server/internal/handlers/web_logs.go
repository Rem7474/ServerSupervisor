package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	weblogssvc "github.com/serversupervisor/server/internal/services/weblogs"
)

// WebLogsHandler serves the /security/web-logs/* endpoints. It translates HTTP
// (admin authz, query parsing/validation, response envelopes); the dispatch +
// reads + summary enrichment live in internal/services/weblogs.
type WebLogsHandler struct {
	svc *weblogssvc.Service
}

func NewWebLogsHandler(svc *weblogssvc.Service) *WebLogsHandler {
	return &WebLogsHandler{svc: svc}
}

// requireWebLogsAdmin returns false (and writes 403) when the caller is not admin.
func (h *WebLogsHandler) requireWebLogsAdmin(c *gin.Context) bool {
	if c.GetString("role") != "admin" {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return false
	}
	return true
}

// validWebLogSource reports whether an (optional) source filter is acceptable.
func validWebLogSource(source string) bool {
	switch source {
	case "", "npm", "nginx", "apache", "caddy":
		return true
	default:
		return false
	}
}

// parseWebLogsPeriod parses ?period (default 24h) into a since timestamp.
func parseWebLogsPeriod(c *gin.Context) (period time.Duration, since time.Time, ok bool) {
	raw := strings.TrimSpace(c.DefaultQuery("period", "24h"))
	period, err := time.ParseDuration(raw)
	if err != nil || period <= 0 {
		respondError(c, apperr.Validation("invalid period (example: 24h, 168h)"))
		return 0, time.Time{}, false
	}
	return period, time.Now().Add(-period), true
}

func webLogsLimit(c *gin.Context, def int) int {
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	return def
}

// BlockCrowdSecIP creates a command for the agent to ban an IP via CrowdSec.
func (h *WebLogsHandler) BlockCrowdSecIP(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	hostID := c.Query("host_id")
	if hostID == "" {
		respondError(c, apperr.Validation("host_id is required"))
		return
	}
	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		respondError(c, apperr.Validation("IP is required"))
		return
	}
	duration := strings.TrimSpace(c.DefaultQuery("duration", "4h"))
	if duration == "" {
		duration = "4h"
	}
	id, err := h.svc.BlockIP(c.Request.Context(), hostID, ip, duration, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": id, "status": "pending"})
}

// UnblockCrowdSecIP creates a command for the agent to unban an IP via CrowdSec.
func (h *WebLogsHandler) UnblockCrowdSecIP(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	hostID := c.Query("host_id")
	if hostID == "" {
		respondError(c, apperr.Validation("host_id is required"))
		return
	}
	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		respondError(c, apperr.Validation("IP is required"))
		return
	}
	id, err := h.svc.UnblockIP(c.Request.Context(), hostID, ip, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"command_id": id, "status": "pending"})
}

func (h *WebLogsHandler) GetWebLogsSummary(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	period, _, ok := parseWebLogsPeriod(c)
	if !ok {
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if !validWebLogSource(source) {
		respondError(c, apperr.Validation("invalid source"))
		return
	}
	result, err := h.svc.Summary(c.Request.Context(), period, hostID, source)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"period":  strings.TrimSpace(c.DefaultQuery("period", "24h")),
		"since":   result.Since,
		"host_id": hostID,
		"source":  source,
		"traffic": result.Traffic,
		"threats": result.Threats,
		"compare": result.Compare,
	})
}

func (h *WebLogsHandler) GetWebLogsIPTimeline(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		respondError(c, apperr.Validation("ip is required"))
		return
	}
	_, since, ok := parseWebLogsPeriod(c)
	if !ok {
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	rows, err := h.svc.IPTimeline(c.Request.Context(), ip, since, hostID, webLogsLimit(c, 500))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ip":       ip,
		"host_id":  hostID,
		"period":   strings.TrimSpace(c.DefaultQuery("period", "24h")),
		"since":    since,
		"count":    len(rows),
		"requests": rows,
	})
}

func (h *WebLogsHandler) GetWebLogsDomainDetails(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	domain := strings.TrimSpace(c.Param("domain"))
	if domain == "" {
		respondError(c, apperr.Validation("domain is required"))
		return
	}
	_, since, ok := parseWebLogsPeriod(c)
	if !ok {
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	data, err := h.svc.DomainDetails(c.Request.Context(), domain, since, hostID, source, webLogsLimit(c, 300))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"domain":  domain,
		"period":  strings.TrimSpace(c.DefaultQuery("period", "24h")),
		"since":   since,
		"host_id": hostID,
		"source":  source,
		"details": data,
	})
}

func (h *WebLogsHandler) GetWebLogsTimeseries(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	_, since, ok := parseWebLogsPeriod(c)
	if !ok {
		return
	}
	bucket := strings.ToLower(strings.TrimSpace(c.DefaultQuery("bucket", "hour")))
	if bucket != "hour" && bucket != "minute" {
		respondError(c, apperr.Validation("invalid bucket (hour|minute)"))
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if !validWebLogSource(source) {
		respondError(c, apperr.Validation("invalid source"))
		return
	}
	points, err := h.svc.Timeseries(c.Request.Context(), since, hostID, source, bucket)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"period":  strings.TrimSpace(c.DefaultQuery("period", "24h")),
		"bucket":  bucket,
		"since":   since,
		"host_id": hostID,
		"source":  source,
		"points":  points,
	})
}

func (h *WebLogsHandler) GetWebLogsLive(c *gin.Context) {
	if !h.requireWebLogsAdmin(c) {
		return
	}
	hostID := strings.TrimSpace(c.Query("host_id"))
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	if !validWebLogSource(source) {
		respondError(c, apperr.Validation("invalid source"))
		return
	}
	rows, err := h.svc.Live(c.Request.Context(), hostID, source, webLogsLimit(c, 100))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"host_id":  hostID,
		"source":   source,
		"count":    len(rows),
		"requests": rows,
	})
}
