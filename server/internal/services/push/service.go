// Package push is the application/service layer for Web Push (VAPID). It owns the
// VAPID key provisioning and subscription persistence behind a Repository port.
package push

import (
	"context"
	"log/slog"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	SavePushSubscription(ctx context.Context, username, endpoint, p256dh, authKey, userAgent string) error
	DeletePushSubscription(ctx context.Context, endpoint string) error
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
