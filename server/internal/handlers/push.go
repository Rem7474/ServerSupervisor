package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	pushsvc "github.com/serversupervisor/server/internal/services/push"
)

// PushHandler manages Web Push (VAPID) subscriptions and VAPID key provisioning.
type PushHandler struct {
	svc *pushsvc.Service
}

func NewPushHandler(svc *pushsvc.Service) *PushHandler {
	return &PushHandler{svc: svc}
}

// GetVapidPublicKey returns the VAPID public key the frontend needs to subscribe.
// GET /api/v1/push/vapid-public-key
func (h *PushHandler) GetVapidPublicKey(c *gin.Context) {
	publicKey, err := h.svc.PublicKey(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "Push: failed to get/generate VAPID keys", slog.Any("err", err))
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": publicKey})
}

// Subscribe saves a Web Push subscription for the authenticated user.
// POST /api/v1/push/subscribe
func (h *PushHandler) Subscribe(c *gin.Context) {
	username := c.GetString("username")
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
		Keys     struct {
			P256DH string `json:"p256dh" binding:"required"`
			Auth   string `json:"auth" binding:"required"`
		} `json:"keys" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid subscription payload"))
		return
	}
	userAgent := c.GetHeader("User-Agent")
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	if err := h.svc.Subscribe(c.Request.Context(), username, req.Endpoint, req.Keys.P256DH, req.Keys.Auth, userAgent); err != nil {
		slog.ErrorContext(c.Request.Context(), "Push: failed to save subscription", slog.String("user", username), slog.Any("err", err))
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Unsubscribe removes a Web Push subscription by its endpoint URL.
// DELETE /api/v1/push/subscribe
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation("invalid request"))
		return
	}
	if err := h.svc.Unsubscribe(c.Request.Context(), req.Endpoint); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
