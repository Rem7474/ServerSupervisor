package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	npmsvc "github.com/serversupervisor/server/internal/services/npm"
)

// NPMPollInterval is the background poller tick — actual per-connection
// poll_interval_sec is enforced inside RefreshAllEnabled.
const NPMPollInterval = 30 * time.Second

// NPMHandler manages Nginx Proxy Manager connections and exposes the import flow.
type NPMHandler struct {
	svc       *npmsvc.Service
	pollerCtx context.Context
}

func NewNPMHandler(svc *npmsvc.Service) *NPMHandler {
	return &NPMHandler{svc: svc, pollerCtx: context.Background()}
}

// SetBackgroundContext threads the SIGTERM-bound root context so fire-and-forget
// goroutines spawned by HTTP requests survive after the request context is cancelled.
func (h *NPMHandler) SetBackgroundContext(ctx context.Context) {
	h.pollerCtx = ctx
}

// PollOnce calls RefreshAllEnabled once; scheduling is owned by poller.Every.
func (h *NPMHandler) PollOnce(ctx context.Context) {
	h.svc.RefreshAllEnabled(ctx)
}

// ─── Connection CRUD ─────────────────────────────────────────────────────────

func (h *NPMHandler) ListConnections(c *gin.Context) {
	conns, err := h.svc.ListConnections(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"connections": conns})
}

func (h *NPMHandler) CreateConnection(c *gin.Context) {
	var req models.NPMConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	conn, err := h.svc.CreateConnection(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, conn)
}

func (h *NPMHandler) UpdateConnection(c *gin.Context) {
	var req models.NPMConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	conn, err := h.svc.UpdateConnection(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, conn)
}

func (h *NPMHandler) DeleteConnection(c *gin.Context) {
	if err := h.svc.DeleteConnection(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Test ─────────────────────────────────────────────────────────────────────

// TestConnection verifies credentials without saving them.
func (h *NPMHandler) TestConnection(c *gin.Context) {
	var req struct {
		APIURL   string `json:"api_url" binding:"required"`
		Identity string `json:"identity" binding:"required"`
		Secret   string `json:"secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if err := h.svc.TestConnection(c.Request.Context(), req.APIURL, req.Identity, req.Secret); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ─── Proxy Host list & refresh ────────────────────────────────────────────────

// ListProxyHosts returns proxy hosts already imported for a connection.
func (h *NPMHandler) ListProxyHosts(c *gin.Context) {
	hosts, err := h.svc.ListProxyHosts(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"proxy_hosts": hosts})
}

// ─── Global proxy host view ───────────────────────────────────────────────────

// ListAllProxyHosts returns every imported proxy host across all connections,
// enriched with connection name and live uptime/SSL status.
func (h *NPMHandler) ListAllProxyHosts(c *gin.Context) {
	hosts, err := h.svc.ListAllProxyHosts(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"proxy_hosts": hosts})
}

// SetNPMEnabled toggles a proxy host's enabled state directly in NPM.
// On success the local DB is updated and monitoring is cascaded off when disabling.
func (h *NPMHandler) SetNPMEnabled(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	result, err := h.svc.SetNPMProxyHostEnabled(c.Request.Context(), c.Param("id"), body.Enabled)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// UpdateProxyHost applies monitoring toggle changes to a single proxy host and
// propagates enable/disable to the linked uptime probe and SSL certificate.
func (h *NPMHandler) UpdateProxyHost(c *gin.Context) {
	var req models.NPMProxyHostUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	result, err := h.svc.UpdateProxyHostMonitoring(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// RefreshNow triggers an immediate background refresh for one connection.
func (h *NPMHandler) RefreshNow(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.svc.GetConnection(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	go func() { _ = h.svc.RefreshSync(h.pollerCtx, id) }()
	c.JSON(http.StatusOK, gin.H{"message": "refresh triggered"})
}
