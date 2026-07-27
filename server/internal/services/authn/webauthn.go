package authn

import (
	"bytes"
	"context"
	"encoding/binary"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

// webauthnSessionTTL bounds how long a begin-registration/begin-login challenge
// stays redeemable — matches the library's own default ceremony timeout order
// of magnitude. Sessions are held in-process only (same pattern as the IP rate
// limiter / WS hubs — see root CLAUDE.md — this is a single-instance deployment).
const webauthnSessionTTL = 5 * time.Minute

// pendingWebAuthnSession pairs the library's SessionData with an expiry the
// service enforces itself, independent of whatever Expires the library set.
type pendingWebAuthnSession struct {
	data      webauthn.SessionData
	expiresAt time.Time
}

// webauthnSessionStore is a small in-process, mutex-guarded map keyed by a
// random opaque token handed to the client between Begin* and Finish* calls.
// It intentionally does not persist across restarts — an in-flight ceremony
// interrupted by a redeploy simply has to be retried, same tradeoff as every
// other in-process ceremony/session state in this server.
type webauthnSessionStore struct {
	mu   sync.Mutex
	data map[string]pendingWebAuthnSession
}

func newWebauthnSessionStore() *webauthnSessionStore {
	return &webauthnSessionStore{data: make(map[string]pendingWebAuthnSession)}
}

func (s *webauthnSessionStore) put(session webauthn.SessionData) (string, error) {
	token, err := generateRefreshToken() // reuse the existing 32-byte random b64 helper
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gc()
	s.data[token] = pendingWebAuthnSession{data: session, expiresAt: time.Now().Add(webauthnSessionTTL)}
	return token, nil
}

func (s *webauthnSessionStore) take(token string) (webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.data[token]
	delete(s.data, token) // single-use: a challenge is redeemed at most once
	if !ok || time.Now().After(entry.expiresAt) {
		return webauthn.SessionData{}, false
	}
	return entry.data, true
}

// gc must be called with mu held.
func (s *webauthnSessionStore) gc() {
	now := time.Now()
	for k, v := range s.data {
		if now.After(v.expiresAt) {
			delete(s.data, k)
		}
	}
}

// webauthnUser adapts a models.User + its credentials to the webauthn.User
// interface the library's ceremonies operate on.
type webauthnUser struct {
	id          []byte
	username    string
	credentials []webauthn.Credential
}

func newWebauthnUser(userID int64, username string, creds []models.WebAuthnCredential) *webauthnUser {
	id := make([]byte, 8)
	binary.BigEndian.PutUint64(id, uint64(userID))
	list := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		list = append(list, c.Credential)
	}
	return &webauthnUser{id: id, username: username, credentials: list}
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.username }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.username }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// buildWebAuthnConfig derives the Relying Party ID + allowed origins from the
// server's own BASE_URL/ALLOWED_ORIGINS rather than a dedicated env var: the
// RP ID must be the effective domain the frontend is served from, which for
// this project is always the same origin the API answers on (see BaseURL's
// doc in config.go). WEBAUTHN_RP_ID/WEBAUTHN_RP_ORIGINS let an operator
// override this for setups where that doesn't hold (e.g. reverse proxy
// rewriting).
func buildWebAuthnConfig(cfg *config.Config) (*webauthn.Config, error) {
	rpID := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID"))
	var origins []string
	if raw := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ORIGINS")); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
	}

	if rpID == "" {
		parsed, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, err
		}
		rpID = parsed.Hostname()
	}
	if len(origins) == 0 {
		origins = append([]string{cfg.BaseURL}, cfg.AllowedOrigins...)
	}

	return &webauthn.Config{
		RPDisplayName: "ServerSupervisor",
		RPID:          rpID,
		RPOrigins:     origins,
	}, nil
}

// initWebAuthn builds the *webauthn.WebAuthn instance for the service. Errors
// are logged rather than fatal — WebAuthn is an optional additional MFA
// method; TOTP keeps working even if e.g. BASE_URL is unparseable.
func initWebAuthn(cfg *config.Config) *webauthn.WebAuthn {
	waCfg, err := buildWebAuthnConfig(cfg)
	if err != nil {
		slog.Error("webauthn config derivation failed — passkey/security-key MFA disabled", slog.Any("err", err))
		return nil
	}
	wa, err := webauthn.New(waCfg)
	if err != nil {
		slog.Error("webauthn.New failed — passkey/security-key MFA disabled", slog.Any("err", err))
		return nil
	}
	return wa
}

func (s *Service) requireWebAuthn() error {
	if s.wa == nil {
		return apperr.Failed("WebAuthn is not available on this server")
	}
	return nil
}

// ===== registration (authenticated user managing their own passkeys) =====

// BeginWebAuthnRegistration starts a "register a new passkey/security key"
// ceremony for the already-authenticated username.
func (s *Service) BeginWebAuthnRegistration(ctx context.Context, username string) (*protocol.CredentialCreation, string, error) {
	if err := s.requireWebAuthn(); err != nil {
		return nil, "", err
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, "", apperr.Unauthorized("unauthorized")
	}
	existing, err := s.repo.ListWebAuthnCredentials(ctx, user.ID)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	waUser := newWebauthnUser(user.ID, user.Username, existing)

	exclude := make([]protocol.CredentialDescriptor, 0, len(existing))
	for _, c := range existing {
		exclude = append(exclude, protocol.CredentialDescriptor{Type: protocol.PublicKeyCredentialType, CredentialID: c.Credential.ID})
	}

	creation, session, err := s.wa.BeginRegistration(waUser,
		webauthn.WithExclusions(exclude),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
			UserVerification: protocol.VerificationPreferred,
		}),
	)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	token, err := s.webauthnSessions.put(*session)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	return creation, token, nil
}

// FinishWebAuthnRegistration completes registration and persists the new
// credential under the given display name.
func (s *Service) FinishWebAuthnRegistration(ctx context.Context, username, sessionToken, name string, rawResponse []byte) (*models.WebAuthnCredential, error) {
	if err := s.requireWebAuthn(); err != nil {
		return nil, err
	}
	session, ok := s.webauthnSessions.take(sessionToken)
	if !ok {
		return nil, apperr.Validation("registration challenge expired, please retry")
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, apperr.Unauthorized("unauthorized")
	}
	existing, err := s.repo.ListWebAuthnCredentials(ctx, user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	waUser := newWebauthnUser(user.ID, user.Username, existing)

	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(rawResponse))
	if err != nil {
		return nil, apperr.Validation("invalid credential response: " + err.Error())
	}
	cred, err := s.wa.CreateCredential(waUser, session, parsed)
	if err != nil {
		return nil, apperr.Validation("credential verification failed: " + err.Error())
	}

	if name == "" {
		name = "Clé de sécurité"
	}
	stored, err := s.repo.CreateWebAuthnCredential(ctx, user.ID, name, *cred)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return stored, nil
}

// ListWebAuthnCredentials returns the display list of a user's registered
// passkeys (never includes the raw credential/public key material).
func (s *Service) ListWebAuthnCredentials(ctx context.Context, username string) ([]models.WebAuthnCredential, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, apperr.Unauthorized("unauthorized")
	}
	creds, err := s.repo.ListWebAuthnCredentials(ctx, user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return creds, nil
}

// DeleteWebAuthnCredential removes one of the current user's own passkeys.
func (s *Service) DeleteWebAuthnCredential(ctx context.Context, username, credentialRowID string) error {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return apperr.Unauthorized("unauthorized")
	}
	if err := s.repo.DeleteWebAuthnCredential(ctx, credentialRowID, user.ID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// ===== login (public — the MFA step of Authenticate) =====

// BeginWebAuthnLogin starts an assertion ceremony for the MFA step of a login
// already past the password check. Re-verifying the password here (cheap,
// same bcrypt call as Authenticate) prevents an unauthenticated caller from
// harvesting a user's registered-credential count/IDs or spamming challenge
// generation without knowing the password first — the same brute-force
// posture as the TOTP path, which never reveals anything before a valid
// password is supplied.
func (s *Service) BeginWebAuthnLogin(ctx context.Context, username, password, ip, userAgent string) (*protocol.CredentialAssertion, string, error) {
	if err := s.requireWebAuthn(); err != nil {
		return nil, "", err
	}
	if s.ipBlocked(ctx, ip) {
		return nil, "", apperr.TooManyRequests("Too many failed login attempts, try again later")
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, "", apperr.Unauthorized("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, "", apperr.Unauthorized("invalid credentials")
	}
	creds, err := s.repo.ListWebAuthnCredentials(ctx, user.ID)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	if len(creds) == 0 {
		return nil, "", apperr.Validation("no security key registered for this account")
	}
	waUser := newWebauthnUser(user.ID, user.Username, creds)

	assertion, session, err := s.wa.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	token, err := s.webauthnSessions.put(*session)
	if err != nil {
		return nil, "", apperr.Internal(err)
	}
	return assertion, token, nil
}

// FinishWebAuthnLogin verifies the assertion and, on success, behaves like the
// tail end of Authenticate: records the login event and returns the user so
// the handler can IssueSession exactly as it would after a valid TOTP code.
func (s *Service) FinishWebAuthnLogin(ctx context.Context, username, sessionToken string, rawResponse []byte, ip, userAgent string) (*models.User, error) {
	if err := s.requireWebAuthn(); err != nil {
		return nil, err
	}
	session, ok := s.webauthnSessions.take(sessionToken)
	if !ok {
		return nil, apperr.Unauthorized("login challenge expired, please retry")
	}
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, apperr.Unauthorized("invalid credentials")
	}
	creds, err := s.repo.ListWebAuthnCredentials(ctx, user.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	waUser := newWebauthnUser(user.ID, user.Username, creds)

	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(rawResponse))
	if err != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, apperr.Unauthorized("invalid credential response")
	}
	updatedCred, err := s.wa.ValidateLogin(waUser, session, parsed)
	if err != nil {
		s.recordFailure(ctx, username, ip, userAgent)
		return nil, apperr.Unauthorized("security key verification failed")
	}
	if err := s.repo.UpdateWebAuthnCredentialUsage(ctx, updatedCred.ID, *updatedCred); err != nil {
		slog.ErrorContext(ctx, "failed to persist webauthn sign-count update", slog.String("user", username), slog.Any("err", err))
	}

	_ = s.repo.CreateLoginEvent(ctx, username, ip, userAgent, true)
	return user, nil
}

// HasWebAuthnCredentials reports whether username has at least one registered
// passkey — used by Authenticate to decide whether the MFA step should be
// offered even for a user with TOTP disabled.
func (s *Service) HasWebAuthnCredentials(ctx context.Context, userID int64) bool {
	if s.wa == nil {
		return false
	}
	n, err := s.repo.CountWebAuthnCredentials(ctx, userID)
	return err == nil && n > 0
}
