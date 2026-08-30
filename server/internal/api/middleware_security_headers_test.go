package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/api"
)

// TestSecurityHeadersMiddleware covers the standard security headers,
// including the CSP style-src 'unsafe-inline' exception documented in
// SecurityHeadersMiddleware's doc comment (ApexCharts drives tooltips/hover
// states via direct style mutation and has no CSP nonce support yet).
func TestSecurityHeadersMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(api.SecurityHeadersMiddleware())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP should scope 'unsafe-inline' to style-src, got %q", csp)
	}
	if !strings.Contains(csp, "script-src 'self';") {
		t.Errorf("CSP script-src should stay strict (no unsafe-inline), got %q", csp)
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP should set frame-ancestors 'none', got %q", csp)
	}
}
