// Package logging provides structured logging for the agent, built on log/slog.
//
// Init wires a text (default) or JSON handler as the slog default and bridges
// the standard library log package through it, so any residual log.Printf call
// site or third-party dependency still produces a structured record. Mirrors the
// server's internal/logging without the request-scoped correlation ID, which has
// no meaning in the agent process.
package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Init configures the global slog logger and redirects the standard log package
// through it. level is one of debug|info|warn|error (default info); format is
// text|json (default text — agents typically log to journald/console). Returns
// the configured logger.
func Init(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(format), "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Bridge the standard library logger through slog so any residual log.Printf
	// (or third-party) output is uniformly structured rather than raw lines.
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

// Discard returns a logger that drops all records. Useful in tests.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
