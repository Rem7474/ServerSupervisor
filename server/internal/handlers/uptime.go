package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/synthetic"
)

type UptimeHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewUptimeHandler(db *database.DB, cfg *config.Config) *UptimeHandler {
	return &UptimeHandler{db: db, cfg: cfg}
}

type uptimeProbePayload struct {
	Name              string `json:"name" binding:"required"`
	Type              string `json:"type" binding:"required,oneof=http tcp"`
	Target            string `json:"target" binding:"required"`
	IntervalSec       int    `json:"interval_sec"`
	TimeoutSec        int    `json:"timeout_sec"`
	ExpectedStatus    int    `json:"expected_status"`
	ExpectedBodyRegex string `json:"expected_body_regex"`
	FollowRedirects   *bool  `json:"follow_redirects"`
	VerifyTLS         *bool  `json:"verify_tls"`
	Enabled           *bool  `json:"enabled"`
}

func (p uptimeProbePayload) toModel() models.UptimeProbe {
	m := models.UptimeProbe{
		Name:              strings.TrimSpace(p.Name),
		Type:              p.Type,
		Target:            strings.TrimSpace(p.Target),
		IntervalSec:       p.IntervalSec,
		TimeoutSec:        p.TimeoutSec,
		ExpectedStatus:    p.ExpectedStatus,
		ExpectedBodyRegex: p.ExpectedBodyRegex,
		FollowRedirects:   true,
		VerifyTLS:         true,
		Enabled:           true,
	}
	if m.IntervalSec < 10 {
		m.IntervalSec = 60
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 10
	}
	if m.Type == "http" && m.ExpectedStatus == 0 {
		m.ExpectedStatus = 200
	}
	if p.FollowRedirects != nil {
		m.FollowRedirects = *p.FollowRedirects
	}
	if p.VerifyTLS != nil {
		m.VerifyTLS = *p.VerifyTLS
	}
	if p.Enabled != nil {
		m.Enabled = *p.Enabled
	}
	return m
}

// List returns all uptime probes.
func (h *UptimeHandler) List(c *gin.Context) {
	probes, err := h.db.ListUptimeProbes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if probes == nil {
		probes = []models.UptimeProbe{}
	}
	c.JSON(http.StatusOK, gin.H{"probes": probes})
}

func (h *UptimeHandler) Get(c *gin.Context) {
	p, err := h.db.GetUptimeProbe(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *UptimeHandler) Create(c *gin.Context) {
	var req uptimeProbePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.db.CreateUptimeProbe(c.Request.Context(), req.toModel())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *UptimeHandler) Update(c *gin.Context) {
	var req uptimeProbePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m := req.toModel()
	m.ID = c.Param("id")
	if err := h.db.UpdateUptimeProbe(c.Request.Context(), m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := h.db.GetUptimeProbe(c.Request.Context(), m.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *UptimeHandler) Delete(c *gin.Context) {
	if err := h.db.DeleteUptimeProbe(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// History returns recent result samples for a probe.
func (h *UptimeHandler) History(c *gin.Context) {
	limit := 200
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	results, err := h.db.GetUptimeProbeResults(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []models.UptimeProbeResult{}
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// Stats returns aggregated uptime % and latency over a time window.
func (h *UptimeHandler) Stats(c *gin.Context) {
	hours := 24
	if v := c.Query("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	stats, err := h.db.GetUptimeStats(c.Request.Context(), c.Param("id"), hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// CheckNow runs a probe immediately and records the result.
func (h *UptimeHandler) CheckNow(c *gin.Context) {
	probe, err := h.db.GetUptimeProbe(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := synthetic.RunOnce(c.Request.Context(), *probe)
	if err := h.db.RecordUptimeProbeResult(c.Request.Context(), result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
