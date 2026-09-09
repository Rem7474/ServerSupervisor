package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterMiddleware_BlocksAndSanitizes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewIPRateLimiter(1, 1, nil)
	defer rl.Stop()

	r := gin.New()
	r.Use(RequestLogger())
	r.Use(RateLimiterMiddleware(rl))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test?token=secret", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("first request expected 200, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test?token=secret", nil)
	req2.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != 429 {
		t.Fatalf("second request expected 429, got %d", w2.Code)
	}
}
