package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// newTestLogger returns a logger writing JSON to buf, wrapped in the same
// contextHandler used in production so request_id enrichment is exercised.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	base := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(&contextHandler{base})
}

func TestContextHandlerInjectsRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	ctx := ContextWithRequestID(context.Background(), "req-123")
	logger.InfoContext(ctx, "hello")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("log line is not valid JSON: %v (%q)", err, buf.String())
	}
	if got := rec["request_id"]; got != "req-123" {
		t.Fatalf("request_id = %v, want req-123", got)
	}
	if got := rec["msg"]; got != "hello" {
		t.Fatalf("msg = %v, want hello", got)
	}
}

func TestContextHandlerOmitsRequestIDWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	logger.Info("no ctx id")

	if strings.Contains(buf.String(), "request_id") {
		t.Fatalf("expected no request_id attr, got: %q", buf.String())
	}
}

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "abc")
	if got := RequestIDFromContext(ctx); got != "abc" {
		t.Fatalf("RequestIDFromContext = %q, want abc", got)
	}
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("RequestIDFromContext on empty ctx = %q, want empty", got)
	}
	if got := RequestIDFromContext(nil); got != "" { //nolint:staticcheck // nil ctx is intentionally tested
		t.Fatalf("RequestIDFromContext(nil) = %q, want empty", got)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
		"bogus": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFromContextFallsBackToDefault(t *testing.T) {
	if FromContext(context.Background()) != slog.Default() {
		t.Fatal("FromContext without stored logger should return slog.Default()")
	}
	custom := Discard()
	ctx := ContextWithLogger(context.Background(), custom)
	if FromContext(ctx) != custom {
		t.Fatal("FromContext should return the stored logger")
	}
}
