package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/api"
)

// captureSlogJSON temporarily replaces the default slog logger with a JSON
// handler writing to buf, restoring the previous default on test cleanup.
func captureSlogJSON(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
}

// TestRequestLogger_AttachesErrorDetail covers RequestLogger's error_detail
// attribute — the field respondError (internal/handlers/httperr.go) records
// via gin's own c.Error so an operator reading docker logs can see *why* a
// request failed, not just its status code. Uses a plain c.Error call
// directly rather than pulling in the handlers package, since RequestLogger
// only ever reads back through gin's own c.Errors — it has no dependency on
// how that got populated.
func TestRequestLogger_AttachesErrorDetail(t *testing.T) {
	var buf bytes.Buffer
	captureSlogJSON(t, &buf)

	r := gin.New()
	r.Use(api.RequestLogger())
	r.GET("/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("dial tcp 10.0.0.5:8006: i/o timeout"))
		c.Status(http.StatusBadGateway)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("expected a single JSON log line, got %q: %v", buf.String(), err)
	}
	if line["error_detail"] != "dial tcp 10.0.0.5:8006: i/o timeout" {
		t.Errorf("expected error_detail to carry the recorded error, got %v", line["error_detail"])
	}
}

// TestRequestLogger_OmitsErrorDetailWhenNoErrorRecorded guards the other
// branch: a plain successful request must not grow a spurious error_detail
// field.
func TestRequestLogger_OmitsErrorDetailWhenNoErrorRecorded(t *testing.T) {
	var buf bytes.Buffer
	captureSlogJSON(t, &buf)

	r := gin.New()
	r.Use(api.RequestLogger())
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("expected a single JSON log line, got %q: %v", buf.String(), err)
	}
	if _, present := line["error_detail"]; present {
		t.Errorf("expected no error_detail field on a successful request, got %v", line["error_detail"])
	}
}
