// Package push is the application/service layer for Web Push (VAPID). It owns the
// VAPID key provisioning and subscription persistence behind a Repository port.
package push

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	SavePushSubscription(ctx context.Context, username, endpoint, p256dh, authKey, userAgent string) error
	DeletePushSubscription(ctx context.Context, endpoint string) error
	GetPushSubscriptionsByRole(ctx context.Context, role string) ([]models.PushSubscription, error)
}

// Service holds the push use-cases.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// PublicKey returns the VAPID public key the frontend needs to subscribe,
// generating and persisting a fresh key pair on first use.
func (s *Service) PublicKey(ctx context.Context) (string, error) {
	_, public, err := s.ensureVapidKeys(ctx)
	return public, err
}

// ensureVapidKeys returns the stored VAPID key pair, generating + persisting one
// on first use (stored as URL-safe base64 under vapid_private_key/vapid_public_key).
func (s *Service) ensureVapidKeys(ctx context.Context) (privateKey, publicKey string, err error) {
	if priv, e := s.repo.GetSetting(ctx, "vapid_private_key"); e == nil && priv != "" {
		if pub, e2 := s.repo.GetSetting(ctx, "vapid_public_key"); e2 == nil && pub != "" {
			return priv, pub, nil
		}
	}
	privateKey, publicKey, err = webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", err
	}
	_ = s.repo.SetSetting(ctx, "vapid_private_key", privateKey)
	_ = s.repo.SetSetting(ctx, "vapid_public_key", publicKey)
	slog.InfoContext(ctx, "Push: generated new VAPID key pair")
	return privateKey, publicKey, nil
}

// Subscribe stores a Web Push subscription for the user.
func (s *Service) Subscribe(ctx context.Context, username, endpoint, p256dh, authKey, userAgent string) error {
	return s.repo.SavePushSubscription(ctx, username, endpoint, p256dh, authKey, userAgent)
}

// Unsubscribe removes a Web Push subscription by its endpoint.
func (s *Service) Unsubscribe(ctx context.Context, endpoint string) error {
	return s.repo.DeletePushSubscription(ctx, endpoint)
}

// Send delivers a Web Push notification to every admin device subscription.
// Shared by every domain (alerts, release trackers, git webhooks) that offers a
// "browser" notification channel, so a native Android/PWA notification is shown
// even when the app is fully closed — not just while a tab is open. Safe to call
// with no VAPID keys or no subscriptions yet (silently returns).
func (s *Service) Send(ctx context.Context, cfg *config.Config, payload map[string]interface{}) {
	privateKey, publicKey, err := s.ensureVapidKeys(ctx)
	if err != nil || privateKey == "" || publicKey == "" {
		return
	}

	data, _ := json.Marshal(payload)

	subs, err := s.repo.GetPushSubscriptionsByRole(ctx, "admin")
	if err != nil || len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DHKey,
				Auth:   sub.AuthKey,
			},
		}
		resp, sendErr := webpush.SendNotification(data, wpSub, &webpush.Options{
			Subscriber:      cfg.BaseURL,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             120,
		})
		if sendErr != nil {
			slog.ErrorContext(ctx, "push: delivery failed", slog.String("endpoint", truncateStr(sub.Endpoint, 40)), slog.Any("err", sendErr))
			if resp != nil && resp.StatusCode == http.StatusGone {
				if delErr := s.repo.DeletePushSubscription(ctx, sub.Endpoint); delErr != nil {
					slog.DebugContext(ctx, "push: failed to prune gone subscription", slog.String("endpoint", truncateStr(sub.Endpoint, 40)), slog.Any("err", delErr))
				}
			}
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
