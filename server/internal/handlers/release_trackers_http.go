package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

func (h *ReleaseTrackerHandler) List(c *gin.Context) {
	trackers, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"trackers": trackers})
}

func (h *ReleaseTrackerHandler) Create(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.ReleaseTrackerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"tracker": created})
}

// ListTrackableContainers returns compose-managed containers without a tracker.
func (h *ReleaseTrackerHandler) ListTrackableContainers(c *gin.Context) {
	containers, err := h.svc.TrackableContainers(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"containers": containers})
}

// CreateBulk creates multiple compose trackers in one call (auto-discovery).
func (h *ReleaseTrackerHandler) CreateBulk(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req struct {
		Trackers []models.ReleaseTrackerRequest `json:"trackers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, results, err := h.svc.CreateBulk(c.Request.Context(), req.Trackers)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": created, "results": results})
}

func (h *ReleaseTrackerHandler) Get(c *gin.Context) {
	t, execs, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tracker": t, "executions": execs})
}

func (h *ReleaseTrackerHandler) Update(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.ReleaseTrackerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if err := h.svc.Update(c.Request.Context(), c.Param("id"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *ReleaseTrackerHandler) Delete(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *ReleaseTrackerHandler) TriggerCheck(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	if err := h.svc.TriggerCheck(c.Request.Context(), h.pollerCtx, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "check scheduled"})
}

// Run manually triggers the tracker's task with the last known release info.
func (h *ReleaseTrackerHandler) Run(c *gin.Context) {
	if role := c.GetString("role"); role != models.RoleAdmin && role != models.RoleOperator {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin or operator role required"})
		return
	}
	if err := h.svc.Run(c.Request.Context(), h.pollerCtx, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "execution scheduled"})
}

func (h *ReleaseTrackerHandler) GetExecutions(c *gin.Context) {
	execs, err := h.svc.Executions(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"executions": execs})
}

func (h *ReleaseTrackerHandler) GetVersionHistory(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > 50 {
				n = 50
			}
			limit = n
		}
	}
	history, err := h.svc.VersionHistory(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"history": history})
}
