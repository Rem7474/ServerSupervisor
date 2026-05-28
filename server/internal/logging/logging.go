// Package logging provides structured logging built on log/slog with
// request-scoped correlation IDs.
//
// Init wires a JSON (production) or text (dev) handler as the slog default and
// bridges the standard library log package through it, so existing log.Printf
// call sites become structured records without per-call changes. A
// contextHandler enriches every record with the request_id carried in the
// context, so logs emitted while handling an HTTP request are automatically
// correlated.
package logging

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	loggerKey
)

// Init configures the global slog logger and redirects the standard log
// package through it. level is one of debug|info|warn|error (default info);
// format is json|text (default json). Returns the configured logger.
func Init(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var base slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		base = slog.NewTextHandler(os.Stderr, opts)
	} else {
		base = slog.NewJSONHandler(os.Stderr, opts)
	}

	logger := slog.New(&contextHandler{base})
	slog.SetDefault(logger)

	// Bridge the standard library logger (used by ~200 existing log.Printf
	// call sites) through slog so all output is uniformly structured.
	log.SetFlags(0)
	log.SetOutput(stdLogBridge{logger})

	return logger
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// contextHandler enriches each record with the request_id stored in the
// context (when present) before delegating to the wrapped handler.
type contextHandler struct {
	slog.Handler
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{h.Handler.WithGroup(name)}
}

// stdLogBridge forwards standard library log output to slog at info level.
// Lines are trimmed of the trailing newline added by the log package.
type stdLogBridge struct {
	logger *slog.Logger
}

func (b stdLogBridge) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	b.logger.Info(msg)
	return len(p), nil
}

// ContextWithRequestID returns a copy of ctx carrying the given request ID.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// ContextWithLogger stores a logger in ctx for later retrieval via FromContext.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext returns the logger stored in ctx, falling back to the default.
// The returned logger still benefits from the contextHandler enrichment when
// used with the same ctx (e.g. l.InfoContext(ctx, ...)).
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// Discard returns a logger that drops all records. Useful in tests.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
