package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	statuspagesvc "github.com/serversupervisor/server/internal/services/statuspage"
)

// StatusPageHandler translates HTTP to the status page service: the admin
// configuration endpoints and the unauthenticated public view.
type StatusPageHandler struct {
	svc *statuspagesvc.Service
	// indexPath is the SPA entry point served for the public /status page.
	indexPath string
}

// NewStatusPageHandler builds the handler; indexPath is the SPA index.html.
func NewStatusPageHandler(svc *statuspagesvc.Service, indexPath string) *StatusPageHandler {
	return &StatusPageHandler{svc: svc, indexPath: indexPath}
}

// GetConfig returns the admin configuration (admin only, see router).
func (h *StatusPageHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// UpdateConfig replaces the configuration (admin only, see router).
func (h *StatusPageHandler) UpdateConfig(c *gin.Context) {
	var req models.StatusPageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	cfg, err := h.svc.UpdateConfig(c.Request.Context(), req, c.GetString("username"), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// Public serves the read-only public view; 404 while the page is disabled.
func (h *StatusPageHandler) Public(c *gin.Context) {
	page, err := h.svc.Public(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=30")
	c.JSON(http.StatusOK, page)
}

// Page serves the SPA entry point for /status: 200 while the page is
// enabled, 404 otherwise (the SPA then renders its own "unavailable" state),
// so a disabled page does not answer like an existing one.
func (h *StatusPageHandler) Page(c *gin.Context) {
	enabled, err := h.svc.Enabled(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	status := http.StatusOK
	if !enabled {
		status = http.StatusNotFound
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	body, readErr := os.ReadFile(h.indexPath)
	if readErr != nil {
		c.String(status, http.StatusText(status))
		return
	}
	c.Data(status, "text/html; charset=utf-8", body)
}
