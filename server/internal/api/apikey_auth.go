package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/logging"
	"github.com/serversupervisor/server/internal/models"
	apikeysvc "github.com/serversupervisor/server/internal/services/apikey"
)

// APIKeyAuthenticator is the slice of apikey.Service the middleware needs; an
// interface so the middleware is unit-testable without a database.
type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, raw string) (*apikeysvc.Principal, error)
}

// apiKeyForbiddenPrefixes are route templates (gin FullPath) closed to API
// keys whatever their scope or owner's role: account and session management,
// the API keys themselves, users, settings/config, and everything that stores
// or returns credentials (integration connections, registry credentials, git
// webhook secrets, agent keys, push subscriptions).
var apiKeyForbiddenPrefixes = []string{
	"/api/v1/auth/",
	"/api/v1/api-keys",
	"/api/v1/users",
	"/api/v1/settings",
	"/api/v1/config",
	"/api/v1/registry-credentials",
	"/api/v1/proxmox/instances",
	"/api/v1/npm/connections",
	"/api/v1/webhooks/git",
	"/api/v1/push/",
}

// apiKeyForbiddenRoutes are exact route templates closed to API keys.
var apiKeyForbiddenRoutes = map[string]bool{
	"/api/v1/hosts/bulk":                      true, // registers hosts and returns their agent keys
	"/api/v1/hosts/:id/rotate-key":            true,
	"/api/v1/hosts/:id/permissions":           true,
	"/api/v1/hosts/:id/permissions/:username": true,
}

// apiKeyForbiddenRoute reports whether a request on this route template with
// this method is refused to an API key. GET /api/v1/hosts (the host list) stays
// open; only its POST (host registration, which returns the new agent key) is
// closed.
func apiKeyForbiddenRoute(method, fullPath string) bool {
	if fullPath == "/api/v1/hosts" {
		return method != http.MethodGet && method != http.MethodHead
	}
	if apiKeyForbiddenRoutes[fullPath] {
		return true
	}
	for _, p := range apiKeyForbiddenPrefixes {
		if fullPath == strings.TrimSuffix(p, "/") || strings.HasPrefix(fullPath, p) {
			return true
		}
	}
	return false
}

// bearerAPIKey returns the API key carried by "Authorization: Bearer ssk_…".
func bearerAPIKey(r *http.Request) (string, bool) {
	scheme, value, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, apikeysvc.KeyPrefix) {
		return "", false
	}
	return value, true
}

// abortAPIError renders a typed error the way handlers.respondError does
// (localized message + i18nKey) and stops the chain.
func abortAPIError(c *gin.Context, e *apperr.Error) {
	body := gin.H{"error": e.Message, "code": e.Code}
	if e.I18nKey != "" {
		lang := apperr.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
		body["error"] = apperr.GetMessage(e.I18nKey, lang, e.Params)
		body["i18nKey"] = e.I18nKey
		if len(e.Params) > 0 {
			body["params"] = e.Params
		}
	}
	c.AbortWithStatusJSON(e.HTTPStatus, body)
}

// SessionOrAPIKeyMiddleware authenticates /api/v1 requests. A request carrying
// "Authorization: Bearer ssk_…" is authenticated as an API key; anything else
// goes through JWTMiddleware (session cookie or JWT bearer) unchanged.
//
// An API key acts as its owner with the owner's current role, so it never
// exceeds what that user may do. On top of that: a read-scoped key only
// reaches GET/HEAD, sensitive routes (apiKeyForbiddenRoute) are refused to
// every key, and each key gets its own token bucket from rl (same rate and
// burst as the per-IP limiter) besides the per-IP one.
func SessionOrAPIKeyMiddleware(cfg *config.Config, keys APIKeyAuthenticator, rl *IPRateLimiter) gin.HandlerFunc {
	jwtAuth := JWTMiddleware(cfg)
	return func(c *gin.Context) {
		raw, ok := bearerAPIKey(c.Request)
		if !ok {
			jwtAuth(c)
			return
		}
		p, err := keys.Authenticate(c.Request.Context(), raw)
		if err != nil {
			e := apperr.From(err)
			if e.HTTPStatus == http.StatusUnauthorized {
				slog.WarnContext(c.Request.Context(), "api key rejected",
					slog.String("path", logging.Sanitize(c.Request.URL.Path)),
					slog.String("client_ip", logging.Sanitize(c.ClientIP())),
					slog.String("reason", e.Message))
			}
			abortAPIError(c, e)
			return
		}
		if apiKeyForbiddenRoute(c.Request.Method, c.FullPath()) {
			abortAPIError(c, apperr.Forbidden("endpoint not available with an API key").I18n(apperr.CodeAPIKeyRouteForbidden, nil))
			return
		}
		if p.Scope != models.APIKeyScopeWrite && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			abortAPIError(c, apperr.Forbidden("read-only API key").I18n(apperr.CodeAPIKeyReadOnly, nil))
			return
		}
		if rl != nil && !rl.Allow("api_key:"+strconv.FormatInt(p.KeyID, 10)) {
			abortAPIError(c, apperr.TooManyRequests("rate limit exceeded").I18n(apperr.CodeAPIKeyRateLimited, nil))
			return
		}
		c.Set("username", p.Username)
		c.Set("role", p.Role)
		c.Set("auth_method", "api_key")
		c.Set("api_key_id", p.KeyID)
		c.Next()
	}
}
