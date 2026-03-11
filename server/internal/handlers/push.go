package handlers

import (
	"log"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// PushHandler manages Web Push (VAPID) subscriptions and VAPID key provisioning.
type PushHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewPushHandler(db *database.DB, cfg *config.Config) *PushHandler {
	return &PushHandler{db: db, cfg: cfg}
}

// ensureVapidKeys returns the stored VAPID key pair, generating and persisting a new one on first use.
// Keys are stored as URL-safe base64 in the settings table under "vapid_private_key" / "vapid_public_key".
func (h *PushHandler) ensureVapidKeys() (privateKey, publicKey string, err error) {
	privateKey, err = h.db.GetSetting("vapid_private_key")
	if err == nil && privateKey != "" {
		publicKey, err = h.db.GetSetting("vapid_public_key")
		if err == nil && publicKey != "" {
			return privateKey, publicKey, nil
		}
	}
	privateKey, publicKey, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	_ = h.db.SetSetting("vapid_private_key", privateKey)
	_ = h.db.SetSetting("vapid_public_key", publicKey)
	log.Println("Push: generated new VAPID key pair")
	return privateKey, publicKey, nil
}

// GetVapidPublicKey returns the VAPID public key that the frontend needs to subscribe.
// GET /api/v1/push/vapid-public-key
func (h *PushHandler) GetVapidPublicKey(c *gin.Context) {
	_, publicKey, err := h.ensureVapidKeys()
	if err != nil {
		log.Printf("Push: failed to get/generate VAPID keys: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get VAPID key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"public_key": publicKey})
}

// Subscribe saves a Web Push subscription for the authenticated user.
// The browser provides endpoint + encrypted keys; we store them for later alert fan-out.
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription payload"})
		return
	}
	userAgent := c.GetHeader("User-Agent")
	if len(userAgent) > 500 {
		userAgent = userAgent[:500]
	}
	if err := h.db.SavePushSubscription(username, req.Endpoint, req.Keys.P256DH, req.Keys.Auth, userAgent); err != nil {
		log.Printf("Push: failed to save subscription for %s: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save subscription"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Unsubscribe removes a Web Push subscription by its endpoint URL.
// Called when the user explicitly revokes push permission in the UI.
// DELETE /api/v1/push/subscribe
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.db.DeletePushSubscription(req.Endpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove subscription"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
