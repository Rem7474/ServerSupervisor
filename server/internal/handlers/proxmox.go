package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/errors"
	"github.com/serversupervisor/server/internal/models"
	proxmoxsvc "github.com/serversupervisor/server/internal/services/proxmox"
)

// ProxmoxHandler translates HTTP to the proxmox service and owns the detached ctx
// used by fire-and-forget poll goroutines. All logic (CRUD, reads, live PVE proxy,
// polling) lives in internal/services/proxmox.
type ProxmoxHandler struct {
	svc       *proxmoxsvc.Service
	pollerCtx context.Context
}

func NewProxmoxHandler(svc *proxmoxsvc.Service) *ProxmoxHandler {
	return &ProxmoxHandler{svc: svc, pollerCtx: context.Background()}
}

// ProxmoxPollInterval is the collection tick (respects per-connection
// poll_interval_sec inside the service).
const ProxmoxPollInterval = 30 * time.Second

// SetBackgroundContext threads a long-lived (SIGTERM-bound) ctx for the
// fire-and-forget goroutines spawned from HTTP requests (e.g. PollNow).
func (h *ProxmoxHandler) SetBackgroundContext(ctx context.Context) {
	h.pollerCtx = ctx
}

// PollOnce collects all enabled connections once (scheduling owned by poller.Every).
func (h *ProxmoxHandler) PollOnce(ctx context.Context) {
	h.svc.PollAll(ctx)
}

// renderProxmoxErr maps a service error to a response. When i18nNodeNotFound is
// set, a 404 renders the localized CodeNodeNotFound payload (preserving the
// original behavior of a few node endpoints); otherwise respondError is used.
func renderProxmoxErr(c *gin.Context, err error, i18nNodeNotFound bool) {
	if i18nNodeNotFound && apperr.From(err).HTTPStatus == http.StatusNotFound {
		lang := errors.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		c.JSON(http.StatusNotFound, errors.NewErrorResponse(errors.CodeNodeNotFound, lang))
		return
	}
	respondError(c, err)
}

// ─── CRUD: Connections ────────────────────────────────────────────────────────

func (h *ProxmoxHandler) ListConnections(c *gin.Context) {
	conns, err := h.svc.ListConnections(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, conns)
}

func (h *ProxmoxHandler) CreateConnection(c *gin.Context) {
	var req models.ProxmoxConnectionRequest
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

func (h *ProxmoxHandler) GetConnection(c *gin.Context) {
	conn, err := h.svc.GetConnection(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, conn)
}

func (h *ProxmoxHandler) UpdateConnection(c *gin.Context) {
	var req models.ProxmoxConnectionRequest
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

func (h *ProxmoxHandler) DeleteConnection(c *gin.Context) {
	if err := h.svc.DeleteConnection(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "connection deleted"})
}

func (h *ProxmoxHandler) TestConnection(c *gin.Context) {
	var req struct {
		APIURL             string `json:"api_url" binding:"required"`
		TokenID            string `json:"token_id" binding:"required"`
		TokenSecret        string `json:"token_secret" binding:"required"`
		InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	if ok, errMsg := h.svc.TestConnection(req.APIURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errMsg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ProxmoxHandler) TestConnectionByID(c *gin.Context) {
	ok, errMsg, err := h.svc.TestConnectionByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": errMsg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PollNow triggers an immediate poll for one connection.
func (h *ProxmoxHandler) PollNow(c *gin.Context) {
	if err := h.svc.TriggerPollByID(c.Request.Context(), h.pollerCtx, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "poll triggered"})
}

// ─── Read-only summary ────────────────────────────────────────────────────────

func (h *ProxmoxHandler) GetSummary(c *gin.Context) {
	summary, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}
