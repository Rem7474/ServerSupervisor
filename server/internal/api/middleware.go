package api

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/database"
	errs "github.com/serversupervisor/server/internal/errors"
	"github.com/serversupervisor/server/internal/logging"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastSeen map[string]time.Time
	rps      rate.Limit
	burst    int
	trusted  []*net.IPNet
	done     chan struct{}
}

func NewIPRateLimiter(rps int, burst int, trustedProxyCIDRs []string) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rps:      rate.Limit(rps),
		burst:    burst,
		trusted:  parseTrustedProxies(trustedProxyCIDRs),
		done:     make(chan struct{}),
	}

	// Cleanup goroutine: remove unused limiters every 10 minutes.
	// Exits cleanly when Stop() is called.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.done:
				return
			}
		}
	}()

	return rl
}

// Stop terminates the background cleanup goroutine.
func (rl *IPRateLimiter) Stop() {
	close(rl.done)
}

func (rl *IPRateLimiter) getClientIP(c *gin.Context) string {
	remoteIP := rl.remoteAddrIP(c)
	if remoteIP == "" {
		return c.Request.RemoteAddr
	}

	if rl.isTrustedProxy(remoteIP) {
		// Check X-Forwarded-For header first (trusted proxy only)
		if forwarded := c.Request.Header.Get("X-Forwarded-For"); forwarded != "" {
			if idx := strings.Index(forwarded, ","); idx > 0 {
				return strings.TrimSpace(forwarded[:idx])
			}
			return strings.TrimSpace(forwarded)
		}

		// Check X-Real-IP header
		if realIP := c.Request.Header.Get("X-Real-IP"); realIP != "" {
			return realIP
		}
	}

	return remoteIP
}

func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, ok := rl.limiters[ip]
	if !ok {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[ip] = limiter
	}
	rl.lastSeen[ip] = time.Now()

	return limiter.Allow()
}

func (rl *IPRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove limiters that have not been seen for more than 10 minutes.
	// Using last-access time avoids the bug where a fresh (never-used) limiter
	// starts with a full token bucket and would be immediately evicted.
	cutoff := time.Now().Add(-10 * time.Minute)
	for ip, t := range rl.lastSeen {
		if t.Before(cutoff) {
			delete(rl.limiters, ip)
			delete(rl.lastSeen, ip)
		}
	}
}

func (rl *IPRateLimiter) remoteAddrIP(c *gin.Context) string {
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

func (rl *IPRateLimiter) isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range rl.trusted {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

func parseTrustedProxies(values []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, entry := range values {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipnet, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			nets = append(nets, ipnet)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets
}

// RateLimiterMiddleware applies per-IP rate limiting
func RateLimiterMiddleware(rl *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := rl.getClientIP(c)

		if !rl.Allow(clientIP) {
			slog.WarnContext(c.Request.Context(), "rate limit blocked",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("client_ip", clientIP))
			c.JSON(429, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequestIDMiddleware assigns a correlation ID to every request. It honours an
// inbound X-Request-ID header (e.g. from a reverse proxy) when present,
// otherwise generates a UUID. The ID is stored on the gin context and the
// request context (so DB calls and slog records inherit it) and echoed back in
// the X-Request-ID response header.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Request = c.Request.WithContext(logging.ContextWithRequestID(c.Request.Context(), id))
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// RequestLogger emits a structured access log per request (sensitive query
// params masked). The request_id is added automatically by the logging
// contextHandler from the request context.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		if query != "" {
			query = maskSensitiveParams(query)
		}

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}
		if status >= 500 {
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "request", toLogAttrs(attrs)...)
		} else if status >= 400 {
			slog.LogAttrs(c.Request.Context(), slog.LevelWarn, "request", toLogAttrs(attrs)...)
		} else {
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "request", toLogAttrs(attrs)...)
		}
	}
}

func toLogAttrs(attrs []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if at, ok := a.(slog.Attr); ok {
			out = append(out, at)
		}
	}
	return out
}

// maskSensitiveParams removes or masks sensitive query parameters like tokens, passwords, keys
func maskSensitiveParams(query string) string {
	sensitiveKeys := map[string]bool{
		"token":    true,
		"api_key":  true,
		"key":      true,
		"password": true,
		"secret":   true,
	}

	parts := strings.Split(query, "&")
	for i, part := range parts {
		if idx := strings.Index(part, "="); idx > 0 {
			key := part[:idx]
			if sensitiveKeys[strings.ToLower(key)] {
				parts[i] = key + "=***MASKED***"
			}
		}
	}
	return strings.Join(parts, "&")
}

// SecurityHeadersMiddleware sets HTTP security headers on all responses.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self' data: blob:; "+
				"connect-src 'self' ws: wss:; "+
				"font-src 'self'; "+
				"frame-ancestors 'none'")
		c.Next()
	}
}

// WSTokenMiddleware performs optional pre-upgrade JWT validation for WebSocket
// routes. The JWT is read from the session cookie (preferred) or from the
// legacy ?token= query parameter. An invalid token aborts the upgrade with
// 401. When neither is present the request passes through and the post-upgrade
// message-based handshake remains the authoritative gate.
func WSTokenMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := cookies.ReadAccessToken(c.Request)
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			c.Next()
			return
		}
		t, err := jwt.Parse(token, func(tok *jwt.Token) (interface{}, error) {
			if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !t.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// CORSMiddleware handles CORS with dynamic origin matching (WebSocket-safe).
// It checks the request Origin against BASE_URL and ALLOWED_ORIGINS so that
// additional front-end origins (e.g. dev server, reverse proxy) are accepted.
func CORSMiddleware(baseURL string, extraOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket upgrades skip CORS headers on purpose: the browser does
		// not send a CORS preflight for the upgrade and gorilla/websocket
		// enforces origin validation post-handshake via Upgrader.CheckOrigin
		// (see ws/base.go: isAllowedOrigin).
		if c.Request.Header.Get("Upgrade") == "websocket" {
			c.Next()
			return
		}

		requestOrigin := c.Request.Header.Get("Origin")
		allowedOrigin := resolveAllowedOrigin(requestOrigin, baseURL, extraOrigins)

		c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Authorization, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Vary", "Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// resolveAllowedOrigin returns the origin to echo in the CORS header.
// If the request origin matches BASE_URL or any ALLOWED_ORIGINS entry it is
// returned as-is; otherwise the BASE_URL is used as the safe fallback.
func resolveAllowedOrigin(requestOrigin, baseURL string, extraOrigins []string) string {
	if requestOrigin == "" {
		return baseURL
	}

	candidates := append([]string{baseURL}, extraOrigins...)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		parsedCandidate, err := url.Parse(candidate)
		parsedRequest, err2 := url.Parse(requestOrigin)
		if err != nil || err2 != nil {
			continue
		}
		if parsedRequest.Host == parsedCandidate.Host {
			return requestOrigin
		}
	}
	return baseURL
}

// JWTMiddleware validates JWT tokens for dashboard access.
// The token is read from the session cookie (browser SPA flow) or from the
// Authorization: Bearer header (curl/scripts/mobile clients).
func JWTMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := cookies.ReadAccessToken(c.Request)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing credentials"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		c.Set("username", claims["sub"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

// AdminOnlyMiddleware rejects requests from non-admin users with 403.
// Must be placed after JWTMiddleware so the "role" context value is set.
func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			lang := errs.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
			c.JSON(http.StatusForbidden, gin.H{"error": errs.GetMessage(errs.CodeAdminRequired, lang)})
			c.Abort()
			return
		}
		c.Next()
	}
}

// HostPermissionMiddleware enforces per-host access control.
// Admins always pass. Non-admins with NO host_permissions entries use their
// global role (backward-compatible). Non-admins WITH entries are restricted
// to the listed hosts; if the requested host is not in their list, 403 is returned.
// requiredLevel: "viewer" (any entry) or "operator" (must have operator entry).
func HostPermissionMiddleware(db *database.DB, requiredLevel string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "admin" {
			c.Next()
			return
		}

		hostID := c.Param("id")
		if hostID == "" {
			c.Next()
			return
		}

		username := c.GetString("username")
		restricted, level, err := db.GetHostAccess(c.Request.Context(), username, hostID)
		if err != nil {
			lang := errs.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))
			c.JSON(http.StatusInternalServerError, gin.H{"error": errs.GetMessage(errs.CodePermissionFailed, lang)})
			c.Abort()
			return
		}

		if !restricted {
			// No host-specific restrictions — global role applies.
			c.Next()
			return
		}

		lang := errs.GetLanguageFromAcceptLanguage(c.GetHeader("Accept-Language"))

		if level == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": errs.GetMessage(errs.CodeHostAccessDenied, lang)})
			c.Abort()
			return
		}

		if requiredLevel == "operator" && level != "operator" {
			c.JSON(http.StatusForbidden, gin.H{"error": errs.GetMessage(errs.CodeOperatorRequired, lang)})
			c.Abort()
			return
		}

		c.Set("host_access_level", level)
		c.Next()
	}
}

// APIKeyMiddleware validates agent API keys.
func APIKeyMiddleware(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(cfg.APIKeyHeader)
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			c.Abort()
			return
		}

		host, err := db.GetHostByAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			c.Abort()
			return
		}

		c.Set("host_id", host.ID)
		c.Set("host", host)
		c.Next()
	}
}
