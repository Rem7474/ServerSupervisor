// Package agentws maintains an optional, additive WebSocket connection from
// the agent to the server, used only to be nudged to poll for pending
// commands immediately (see the server's internal/ws.AgentHub) instead of
// waiting out the regular report_interval ticker. It never carries command
// content over the wire — a "poll_now" nudge just triggers the agent's
// existing, atomic poll/claim pipeline sooner, rather than opening a second
// delivery path that could race it. If the connection can't be established
// or drops (older server, a proxy that blocks WebSocket upgrades, a network
// hiccup), the agent's normal poll cycle is completely unaffected — this
// package only ever asks for polls to happen sooner, never fewer or
// differently.
package agentws

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/agent/internal/config"
)

const (
	heartbeatInterval = 20 * time.Second
	dialTimeout       = 10 * time.Second
	minBackoff        = 3 * time.Second
	maxBackoff        = 60 * time.Second
)

type message struct {
	Type string `json:"type"`
}

// Run connects and reconnects (with backoff) for as long as ctx is not
// cancelled. On each "poll_now" message from the server it calls pollNow —
// a non-blocking signal into the agent's existing report loop; this package
// never calls the server's report/command endpoints itself, so report
// submissions stay serialized in the single main-loop goroutine that
// already owns them.
func Run(ctx context.Context, cfg *config.Config, pollNow func()) {
	if cfg.DisableWSPush {
		return
	}
	wsURL, err := toWebSocketURL(cfg.ServerURL)
	if err != nil {
		slog.Warn("agentws: invalid server_url, low-latency command push disabled", "err", err)
		return
	}

	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		if runOnce(ctx, wsURL, cfg.APIKey, cfg.InsecureSkipVerify, pollNow) {
			backoff = minBackoff
		} else {
			backoff = nextBackoff(backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// runOnce dials, authenticates via the same X-API-Key header the agent
// already sends on every HTTP call, then reads until the connection drops or
// ctx is cancelled. Returns whether the dial itself succeeded, so the caller
// can reset its backoff after any healthy session rather than only after a
// long-lived one.
func runOnce(ctx context.Context, wsURL, apiKey string, insecureSkipVerify bool, pollNow func()) bool {
	dialer := websocket.Dialer{
		HandshakeTimeout: dialTimeout,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec // operator opt-in, mirrors sender.Sender's existing transport
	}
	header := http.Header{}
	header.Set("X-API-Key", apiKey)

	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		slog.Debug("agentws: connect failed, will retry", "err", err)
		return false
	}
	defer func() { _ = conn.Close() }()
	slog.Info("agentws: connected — low-latency command push active")

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var msg message
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Type == "poll_now" {
				pollNow()
			}
		}
	}()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return true
		case <-done:
			slog.Debug("agentws: disconnected, will reconnect")
			return true
		case <-heartbeat.C:
			// Sole writer on this connection (the read loop above never
			// writes), so this needs no extra locking beyond gorilla's
			// documented "one reader + one writer" concurrency guarantee.
			if err := conn.WriteJSON(message{Type: "heartbeat"}); err != nil {
				return true
			}
		}
	}
}

func toWebSocketURL(serverURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported server_url scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/agent/ws"
	return u.String(), nil
}

func nextBackoff(cur time.Duration) time.Duration {
	next := cur * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}
