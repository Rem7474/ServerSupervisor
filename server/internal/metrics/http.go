package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// unmatchedRoute labels requests that hit no registered route (SPA fallback,
// scanners probing random paths) so they cannot mint one series per URL.
const unmatchedRoute = "unmatched"

// Middleware records request count, latency and in-flight gauge. The route
// label is gin's route template (e.g. /api/v1/hosts/:id), never the raw path.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpInFlight.Inc()
		start := time.Now()
		c.Next()
		httpInFlight.Dec()

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}
		method := c.Request.Method
		httpRequests.WithLabelValues(method, route, statusClass(c.Writer.Status())).Inc()
		httpDuration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
	}
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

// Handler serves the registry in Prometheus text format.
//
// It is never public: with enabled=false it answers 404 (indistinguishable
// from an absent endpoint), and with no token configured it also answers 404
// rather than serving unauthenticated. Otherwise a matching
// "Authorization: Bearer <token>" is required, compared in constant time.
func Handler(enabled bool, token string) gin.HandlerFunc {
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{DisableCompression: true})
	want := sha256.Sum256([]byte(token))
	return func(c *gin.Context) {
		if !enabled || token == "" {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "endpoint not found", "code": "NOT_FOUND"})
			return
		}
		got := sha256.Sum256([]byte(bearerToken(c.GetHeader("Authorization"))))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="metrics"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func bearerToken(header string) string {
	const prefix = "bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}
