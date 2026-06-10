package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/models"
	uptimesvc "github.com/serversupervisor/server/internal/services/uptime"
)

// UptimeHandler translates HTTP to the uptime service; all probe logic lives in
// the service layer (internal/services/uptime).
type UptimeHandler struct {
	svc *uptimesvc.Service
}

func NewUptimeHandler(svc *uptimesvc.Service) *UptimeHandler {
	return &UptimeHandler{svc: svc}
}

// List returns all uptime probes.
func (h *UptimeHandler) List(c *gin.Context) {
	probes, err := h.svc.ListProbes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"probes": probes})
}

func (h *UptimeHandler) Get(c *gin.Context) {
	p, err := h.svc.GetProbe(c.Request.Context(), c.Param("id"))
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
	var req models.UptimeProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.CreateProbe(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *UptimeHandler) Update(c *gin.Context) {
	var req models.UptimeProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	out, err := h.svc.UpdateProbe(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *UptimeHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteProbe(c.Request.Context(), c.Param("id")); err != nil {
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
	results, err := h.svc.History(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	stats, err := h.svc.Stats(c.Request.Context(), c.Param("id"), hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// CheckNow runs a probe immediately and records the result.
func (h *UptimeHandler) CheckNow(c *gin.Context) {
	result, err := h.svc.CheckNow(c.Request.Context(), c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
