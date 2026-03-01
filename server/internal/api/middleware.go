package api

import (
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	lastSeen map[string]time.Time
	rps      rate.Limit
	burst    int
	trusted  []*net.IPNet
}

func NewIPRateLimiter(rps int, burst int, trustedProxyCIDRs []string) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		lastSeen: make(map[string]time.Time),
		rps:      rate.Limit(rps),
		burst:    burst,
		trusted:  parseTrustedProxies(trustedProxyCIDRs),
	}

	// Cleanup goroutine: remove unused limiters every 10 minutes
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
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
			c.JSON(429, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequestLogger logs all requests (with sensitive info masked)
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Mask sensitive query parameters in logs
		if query != "" {
			query = maskSensitiveParams(query)
			path = path + "?" + query
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[%d] %s %s (%v)", status, c.Request.Method, path, latency)
	}
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

// CORSMiddleware handles CORS with dynamic origin matching (WebSocket-safe).
// It checks the request Origin against BASE_URL and ALLOWED_ORIGINS so that
// additional front-end origins (e.g. dev server, reverse proxy) are accepted.
func CORSMiddleware(baseURL string, extraOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket upgrades handle their own origin check via the upgrader.
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
