package handlers

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	gitwebhooksvc "github.com/serversupervisor/server/internal/services/gitwebhook"
)

// GitWebhookHandler translates HTTP to the git-webhook service. Admin authz and
// the raw-body read for the public receiver stay here; CRUD, HMAC verification,
// payload parsing, dispatch and completion notifications live in
// internal/services/gitwebhook.
type GitWebhookHandler struct {
	svc *gitwebhooksvc.Service
}

func NewGitWebhookHandler(svc *gitwebhooksvc.Service) *GitWebhookHandler {
	return &GitWebhookHandler{svc: svc}
}

// SetBackgroundContext threads a long-lived ctx into the service for fire-and-forget
// completion callbacks. Called once from main.go.
func (h *GitWebhookHandler) SetBackgroundContext(ctx context.Context) {
	h.svc.SetBackgroundContext(ctx)
}

// ========== CRUD (authenticated, admin only) ==========

func (h *GitWebhookHandler) ListWebhooks(c *gin.Context) {
	webhooks, err := h.svc.List(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

func (h *GitWebhookHandler) CreateWebhook(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.GitWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	created, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"webhook": created})
}

func (h *GitWebhookHandler) GetWebhook(c *gin.Context) {
	wh, execs, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"webhook": wh, "executions": execs})
}

func (h *GitWebhookHandler) UpdateWebhook(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	var req models.GitWebhookRequest
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

func (h *GitWebhookHandler) DeleteWebhook(c *gin.Context) {
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

func (h *GitWebhookHandler) RegenerateSecret(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	secret, err := h.svc.RegenerateSecret(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *GitWebhookHandler) GetWebhookExecutions(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	execs, err := h.svc.Executions(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"executions": execs})
}

// ========== Public receiver ==========

// ReceiveWebhook is the public endpoint called by Git providers.
func (h *GitWebhookHandler) ReceiveWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 5<<20)) // 5MB limit
	if err != nil {
		respondError(c, apperr.Validation("failed to read body"))
		return
	}
	res, err := h.svc.Receive(c.Request.Context(), c.Param("id"), body, c.Request.Header, c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}
	resp := gin.H{"status": res.Status}
	if res.Reason != "" {
		resp["reason"] = res.Reason
	}
	if res.ExecutionID != "" {
		resp["execution_id"] = res.ExecutionID
	}
	if res.CommandID != "" {
		resp["command_id"] = res.CommandID
	}
	c.JSON(http.StatusOK, resp)
}

// HandleCommandCompletion implements CommandCompletionListener: it notifies the
// service when a webhook-triggered command reaches a terminal state.
func (h *GitWebhookHandler) HandleCommandCompletion(commandID, status string) {
	h.svc.NotifyComplete(commandID, status)
}
