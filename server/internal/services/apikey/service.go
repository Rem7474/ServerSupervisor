// Package apikey is the service layer for personal API keys: creation (the
// secret is shown once and only its SHA-256 is stored), listing, revocation and
// the authentication of a presented key. HTTP concerns (which routes a key may
// reach, read-only scope, rate limiting) live in internal/api.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

const (
	// KeyPrefix marks a ServerSupervisor API key, so an Authorization: Bearer
	// value can be told apart from a JWT without parsing it.
	KeyPrefix = "ssk_"
	// idLen is the length of the public id that follows KeyPrefix (base32,
	// 40 bits): "ssk_" + id is the lookup/display prefix.
	idLen = 8
	// secretBytes of entropy (256 bits), base64url-encoded without padding.
	secretBytes = 32
	secretLen   = 43

	MaxNameLength        = 100
	MaxExpiresInDays     = 3650
	MaxActiveKeysPerUser = 25
)

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	CreateAPIKey(ctx context.Context, userID int64, name, prefix, keyHash, scope string, expiresAt *time.Time) (*models.APIKey, error)
	GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error)
	ListAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error)
	ListAllAPIKeys(ctx context.Context) ([]models.APIKey, error)
	CountActiveAPIKeys(ctx context.Context, userID int64) (int, error)
	RevokeAPIKey(ctx context.Context, id int64) (bool, error)
	GetAPIKeyForAuth(ctx context.Context, prefix string) (*database.APIKeyAuthRecord, error)
	TouchAPIKeyLastUsed(ctx context.Context, id int64) error
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
}

// Principal is the identity a valid key authenticates as.
type Principal struct {
	KeyID    int64
	Username string
	Role     string
	Scope    string
}

// Service holds the API-key use-cases.
type Service struct {
	repo   Repository
	now    func() time.Time
	random func([]byte) (int, error)
}

// NewService wires the service with the system clock and crypto/rand.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, random: rand.Read}
}

// HashKey returns the hex SHA-256 of a full key. A key carries 256 bits of
// entropy, so a fast unsalted hash is enough: brute-forcing it is infeasible.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ParseKey checks a presented key's shape and returns its lookup prefix
// ("ssk_" + id). It does no I/O.
func ParseKey(raw string) (string, bool) {
	if !strings.HasPrefix(raw, KeyPrefix) || len(raw) != len(KeyPrefix)+idLen+1+secretLen {
		return "", false
	}
	rest := raw[len(KeyPrefix):]
	id, secret := rest[:idLen], rest[idLen+1:]
	if rest[idLen] != '_' {
		return "", false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '2' || r > '7') {
			return "", false
		}
	}
	if _, err := base64.RawURLEncoding.DecodeString(secret); err != nil {
		return "", false
	}
	return KeyPrefix + id, true
}

// generate returns a new key and its lookup prefix.
func (s *Service) generate() (key, prefix string, err error) {
	idRaw := make([]byte, 5)
	if _, err := s.random(idRaw); err != nil {
		return "", "", err
	}
	secret := make([]byte, secretBytes)
	if _, err := s.random(secret); err != nil {
		return "", "", err
	}
	prefix = KeyPrefix + strings.ToLower(idEncoding.EncodeToString(idRaw))
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(secret), prefix, nil
}

func (s *Service) owner(ctx context.Context, username string) (*models.User, error) {
	u, err := s.repo.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.Unauthorized("unknown user").I18n(apperr.CodeAuthRequired, nil)
	}
	return u, err
}

// Create issues a key for username. The returned Secret is the only time the
// full key is available.
func (s *Service) Create(ctx context.Context, username, clientIP string, req models.APIKeyCreateRequest) (*models.APIKeyCreated, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLength {
		return nil, apperr.Validation("invalid key name").
			I18n(apperr.CodeAPIKeyNameInvalid, map[string]string{"max": strconv.Itoa(MaxNameLength)})
	}
	if req.Scope != models.APIKeyScopeRead && req.Scope != models.APIKeyScopeWrite {
		return nil, apperr.Validation("invalid scope").I18n(apperr.CodeAPIKeyScopeInvalid, nil)
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > MaxExpiresInDays {
		return nil, apperr.Validation("invalid expiry").
			I18n(apperr.CodeAPIKeyExpiryInvalid, map[string]string{"max": strconv.Itoa(MaxExpiresInDays)})
	}
	u, err := s.owner(ctx, username)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.CountActiveAPIKeys(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if active >= MaxActiveKeysPerUser {
		return nil, apperr.Conflict("too many active API keys").
			I18n(apperr.CodeAPIKeyLimitReached, map[string]string{"max": strconv.Itoa(MaxActiveKeysPerUser)})
	}
	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := s.now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	key, prefix, err := s.generate()
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("generate api key: %w", err))
	}
	created, err := s.repo.CreateAPIKey(ctx, u.ID, name, prefix, HashKey(key), req.Scope, expiresAt)
	if err != nil {
		return nil, err
	}
	details := fmt.Sprintf("name=%q prefix=%s scope=%s expires_in_days=%d", name, prefix, req.Scope, req.ExpiresInDays)
	_, _ = s.repo.CreateAuditLog(ctx, username, "api_key_create", "", clientIP, details, "success")
	return &models.APIKeyCreated{Key: *created, Secret: key}, nil
}

// ListMine returns the caller's keys (never nil).
func (s *Service) ListMine(ctx context.Context, username string) ([]models.APIKey, error) {
	u, err := s.owner(ctx, username)
	if err != nil {
		return nil, err
	}
	return s.repo.ListAPIKeysByUser(ctx, u.ID)
}

// ListAll returns every key; the handler restricts it to admins.
func (s *Service) ListAll(ctx context.Context) ([]models.APIKey, error) {
	return s.repo.ListAllAPIKeys(ctx)
}

// Revoke revokes a key owned by actor, or any key when actor is an admin.
// Someone else's key answers NotFound to a non-admin, so ids are not probed.
// Revoking an already revoked key succeeds without a second audit entry.
func (s *Service) Revoke(ctx context.Context, actor, actorRole, clientIP string, id int64) error {
	notFound := apperr.NotFound("api key not found").I18n(apperr.CodeAPIKeyNotFound, nil)
	k, err := s.repo.GetAPIKeyByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound
	}
	if err != nil {
		return err
	}
	if k.Username != actor && actorRole != models.RoleAdmin {
		return notFound
	}
	revoked, err := s.repo.RevokeAPIKey(ctx, id)
	if err != nil {
		return err
	}
	if revoked {
		details := fmt.Sprintf("name=%q prefix=%s owner=%s", k.Name, k.Prefix, k.Username)
		_, _ = s.repo.CreateAuditLog(ctx, actor, "api_key_revoke", "", clientIP, details, "success")
	}
	return nil
}

// Authenticate validates a presented key and returns who it acts as, with the
// owner's current role. Every failure is an apperr.Unauthorized.
func (s *Service) Authenticate(ctx context.Context, raw string) (*Principal, error) {
	invalid := apperr.Unauthorized("invalid API key").I18n(apperr.CodeAPIKeyInvalid, nil)
	prefix, ok := ParseKey(raw)
	if !ok {
		return nil, invalid
	}
	rec, err := s.repo.GetAPIKeyForAuth(ctx, prefix)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(HashKey(raw)), []byte(rec.KeyHash)) != 1 {
		return nil, invalid
	}
	if rec.RevokedAt != nil {
		return nil, apperr.Unauthorized("API key revoked").I18n(apperr.CodeAPIKeyRevoked, nil)
	}
	if rec.ExpiresAt != nil && !s.now().Before(*rec.ExpiresAt) {
		return nil, apperr.Unauthorized("API key expired").I18n(apperr.CodeAPIKeyExpired, nil)
	}
	// last_used_at is informational: failing to record it must not turn a
	// valid key away.
	if err := s.repo.TouchAPIKeyLastUsed(ctx, rec.ID); err != nil {
		slog.WarnContext(ctx, "api key: could not record last use", slog.Int64("api_key_id", rec.ID), slog.Any("error", err))
	}
	return &Principal{KeyID: rec.ID, Username: rec.Username, Role: rec.Role, Scope: rec.Scope}, nil
}
