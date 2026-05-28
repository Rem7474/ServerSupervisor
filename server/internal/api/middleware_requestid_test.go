package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/logging"
)

func TestRequestIDMiddlewareGeneratesID(t *testing.T) {
	r := gin.New()
	r.Use(api.RequestIDMiddleware())

	var ctxID, ginID string
	r.GET("/", func(c *gin.Context) {
		ginID = c.GetString("request_id")
		ctxID = logging.RequestIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	header := w.Header().Get("X-Request-ID")
	if header == "" {
		t.Fatal("X-Request-ID response header not set")
	}
	if ginID != header || ctxID != header {
		t.Fatalf("request_id mismatch: header=%q gin=%q ctx=%q", header, ginID, ctxID)
	}
}

func TestRequestIDMiddlewareHonoursInboundHeader(t *testing.T) {
	r := gin.New()
	r.Use(api.RequestIDMiddleware())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "inbound-42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "inbound-42" {
		t.Fatalf("X-Request-ID = %q, want inbound-42", got)
	}
}
