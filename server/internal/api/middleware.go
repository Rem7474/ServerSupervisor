package api

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func NewIPRateLimiter(rps int, burst int) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
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
	// Check X-Forwarded-For header first (for proxies)
	if forwarded := c.Request.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP if multiple are present
		if idx := strings.Index(forwarded, ","); idx > 0 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return strings.TrimSpace(forwarded)
	}

	// Check X-Real-IP header
	if realIP := c.Request.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr (strip port if present)
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil {
		return host
	}
	return c.Request.RemoteAddr
}

func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, ok := rl.limiters[ip]
	if !ok {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[ip] = limiter
	}

	return limiter.Allow()
}

func (rl *IPRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove limiters that are not being actively used (still have tokens available)
	// We check the available tokens without consuming them
	for ip, limiter := range rl.limiters {
		// Check if the limiter has fully recovered (enough tokens for a new burst)
		// This doesn't consume tokens - it just checks the state
		if limiter.Tokens() >= float64(rl.burst) {
			// Limiter is not being used recently - safe to remove
			delete(rl.limiters, ip)
		}
	}
}

// RateLimiterMiddleware applies per-IP rate limiting (but NOT for WebSocket upgrades)
func RateLimiterMiddleware(rl *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting for WebSocket upgrades - they handle their own protocol
		if c.Request.Header.Get("Upgrade") == "websocket" {
			c.Next()
			return
		}

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

// CORSMiddleware handles CORS with proper origin (WebSocket-safe)
func CORSMiddleware(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CORS handling for WebSocket upgrades (they have their own protocol)
		if c.Request.Header.Get("Upgrade") == "websocket" {
			c.Next()
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Authorization, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
