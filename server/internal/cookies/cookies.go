// Package cookies centralises auth cookie names, helpers, and CSRF logic so
// both the api middleware and the handlers package can use them without
// creating an import cycle.
package cookies

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
)

// Cookie names used for the browser session.
//
// AccessToken — short-lived JWT (cfg.JWTExpiration). httpOnly so XSS cannot
// exfiltrate it; SameSite=Lax to forbid cross-site automatic submission.
//
// RefreshToken — long-lived opaque token (cfg.RefreshTokenExpiration), stored
// hashed in DB. httpOnly and scoped to /api/auth so it is never sent on
// regular API calls.
//
// CSRFToken — random per-session value. NOT httpOnly so the SPA can read it
// and echo it back in the X-CSRF-Token header (double-submit pattern).
const (
	AccessTokenName  = "ss_access"
	RefreshTokenName = "ss_refresh"
	CSRFTokenName    = "ss_csrf"
	CSRFHeaderName   = "X-CSRF-Token"
)

// GenerateCSRFToken returns a 32-byte URL-safe random string.
func GenerateCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// secure decides whether the Secure attribute should be set. TLS at the
// server, or an https BASE_URL behind a TLS-terminating proxy, both qualify.
func secure(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.TLSEnabled {
		return true
	}
	if u, err := url.Parse(cfg.BaseURL); err == nil {
		return u.Scheme == "https"
	}
	return false
}

// BasePath returns the path used for the access + CSRF cookies.
func BasePath() string { return "/" }

// RefreshPath restricts the refresh cookie to the auth endpoints.
func RefreshPath() string { return "/api/auth" }

// SetAccess writes the access JWT cookie and matching CSRF token cookie.
// Pass an empty csrfToken to keep the current one (none is written).
func SetAccess(c *gin.Context, cfg *config.Config, accessToken string, expiresAt time.Time, csrfToken string) {
	sec := secure(cfg)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AccessTokenName,
		Value:    accessToken,
		Path:     BasePath(),
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   sec,
		SameSite: http.SameSiteLaxMode,
	})
	if csrfToken != "" {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     CSRFTokenName,
			Value:    csrfToken,
			Path:     BasePath(),
			Expires:  expiresAt,
			MaxAge:   int(time.Until(expiresAt).Seconds()),
			HttpOnly: false, // JS must read this and copy it into the X-CSRF-Token header
			Secure:   sec,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// SetRefresh writes the refresh-token cookie, scoped to the auth endpoints.
func SetRefresh(c *gin.Context, cfg *config.Config, refreshToken string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshTokenName,
		Value:    refreshToken,
		Path:     RefreshPath(),
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   secure(cfg),
		SameSite: http.SameSiteLaxMode,
	})
}

// Clear overwrites the auth cookies with empty, expired values. Used on
// logout and on auth failures so the browser drops them immediately.
func Clear(c *gin.Context, cfg *config.Config) {
	sec := secure(cfg)
	past := time.Unix(0, 0)
	for _, item := range []struct {
		name     string
		path     string
		httpOnly bool
	}{
		{AccessTokenName, BasePath(), true},
		{RefreshTokenName, RefreshPath(), true},
		{CSRFTokenName, BasePath(), false},
	} {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     item.name,
			Value:    "",
			Path:     item.path,
			Expires:  past,
			MaxAge:   -1,
			HttpOnly: item.httpOnly,
			Secure:   sec,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// ReadAccessToken returns the JWT presented by the client, preferring the
// session cookie and falling back to the Authorization: Bearer header so
// curl/scripts/agents keep working.
func ReadAccessToken(r *http.Request) string {
	if ck, err := r.Cookie(AccessTokenName); err == nil && ck.Value != "" {
		return ck.Value
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// ReadRefreshToken returns the refresh token from the cookie, or empty.
func ReadRefreshToken(r *http.Request) string {
	if ck, err := r.Cookie(RefreshTokenName); err == nil && ck.Value != "" {
		return ck.Value
	}
	return ""
}

// HasSessionCookie reports whether the request is being made by a
// cookie-authenticated browser session (vs. a Bearer-token client).
// Used by the CSRF middleware to skip the check when no session is in use.
func HasSessionCookie(r *http.Request) bool {
	ck, err := r.Cookie(AccessTokenName)
	return err == nil && ck.Value != ""
}

// CSRFMiddleware enforces the double-submit cookie pattern on state-changing
// requests. The SPA must echo the CSRF cookie value into the X-CSRF-Token
// header. Bearer-only clients (no session cookie) skip the check.
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if !HasSessionCookie(c.Request) {
			c.Next()
			return
		}
		csrfCookie, err := c.Request.Cookie(CSRFTokenName)
		if err != nil || csrfCookie.Value == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing CSRF token"})
			return
		}
		header := c.GetHeader(CSRFHeaderName)
		if header == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing CSRF header"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}
		c.Next()
	}
}
