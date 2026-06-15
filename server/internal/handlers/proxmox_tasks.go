package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/errors"
)

// ListTasks returns recent tasks, optionally filtered by ?connection_id and ?limit.
func (h *ProxmoxHandler) ListTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	tasks, err := h.svc.ListTasks(c.Request.Context(), c.Query("connection_id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// ListNodeTasks returns recent tasks for a node, optionally limited by ?limit.
func (h *ProxmoxHandler) ListNodeTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	tasks, err := h.svc.ListNodeTasks(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		renderProxmoxErr(c, err, true) // localized CodeNodeNotFound on 404
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// ListBackupJobs returns backup job configs, optionally filtered by ?connection_id.
func (h *ProxmoxHandler) ListBackupJobs(c *gin.Context) {
	jobs, err := h.svc.ListBackupJobs(c.Request.Context(), c.Query("connection_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// ListBackupRuns returns the latest backup result per VM, optionally by ?connection_id.
func (h *ProxmoxHandler) ListBackupRuns(c *gin.Context) {
	runs, err := h.svc.ListBackupRuns(c.Request.Context(), c.Query("connection_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, runs)
}

// GetTaskLog proxies GET /nodes/{node}/tasks/{upid}/log from PVE.
func (h *ProxmoxHandler) GetTaskLog(c *gin.Context) {
	upid := c.Param("upid")
	if upid == "" {
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusBadRequest, errors.NewErrorResponse(errors.CodeMissingParameter, lang))
		return
	}
	lines, err := h.svc.TaskLog(c.Request.Context(), c.Param("id"), upid)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lines)
}

// GetNodeSyslog proxies GET /nodes/{node}/syslog from PVE.
// Supports ?limit (default 200), optional ?search filtering, ?service (default pveproxy).
func (h *ProxmoxHandler) GetNodeSyslog(c *gin.Context) {
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	service := "pveproxy"
	if raw, ok := c.GetQuery("service"); ok {
		service = strings.TrimSpace(raw) // explicit empty means "all services"
	}
	lines, err := h.svc.NodeSyslog(c.Request.Context(), c.Param("id"), limit, service, strings.TrimSpace(c.Query("search")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, lines)
}
