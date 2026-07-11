// Package notifychannels centralizes the "for each configured channel, send"
// fan-out shared by alerts, git webhooks and release trackers. Before this
// package existed, each of those three domains carried its own near-identical
// switch over rule.Actions.Channels / t.NotifyChannels (smtp/ntfy/browser),
// so the dispatch logic could silently drift between them. Now there is one
// implementation; each domain only builds the Event describing what to send.
package notifychannels

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/services/push"
)

// Event describes one notification to fan out across a rule/tracker/webhook's
// configured channels. Only the fields relevant to the channels actually
// listed in Channels need to be set.
type Event struct {
	// LogID identifies the source in log lines (e.g. "rule_id=3", "tracker=abc").
	LogID string

	Channels []string

	SMTPSubject string
	SMTPBody    string
	// SMTPTo is the already-resolved recipient (per-rule override, else the
	// configured default) — callers resolve the fallback themselves since only
	// alert rules support a per-rule override.
	SMTPTo string

	NtfyTitle string
	NtfyBody  string
	// NtfyURL is the already-resolved target URL, same reasoning as SMTPTo.
	NtfyURL string

	// LegacyWebhook, when non-nil, is POSTed as JSON to cfg.NotifyURL on the
	// deprecated "notify" channel (alert rules only — git webhooks and release
	// trackers never put "notify" in Channels).
	LegacyWebhook interface{}

	// OnBrowser fires the domain-specific WebSocket broadcast (different
	// message shape per domain, so it stays a caller-supplied callback).
	OnBrowser func()
	// Push, when non-nil, is delivered as a Web Push notification alongside
	// OnBrowser so the event reaches a closed/backgrounded app too.
	Push *push.Payload
}

// Dispatcher owns the shared cfg/notifier/push dependencies needed to fan an
// Event out. Safe to construct with a nil pushSvc — push sends are then
// skipped (matches the previous per-domain "if s.notifHub == nil" guards).
type Dispatcher struct {
	cfg      *config.Config
	notifier notify.Notifier
	pushSvc  *push.Service
}

func NewDispatcher(cfg *config.Config, pushSvc *push.Service) *Dispatcher {
	return &Dispatcher{cfg: cfg, notifier: notify.New(), pushSvc: pushSvc}
}

// Send fans ev out across every channel it names. A channel left
// unconfigured (missing SMTP/ntfy destination) is logged and skipped rather
// than failing the whole event.
func (d *Dispatcher) Send(ctx context.Context, ev Event) {
	for _, ch := range ev.Channels {
		switch ch {
		case "smtp":
			if ev.SMTPTo == "" || d.cfg.SMTPFrom == "" {
				slog.WarnContext(ctx, "notifychannels: SMTP to/from not configured", slog.String("source", ev.LogID))
				continue
			}
			if err := d.notifier.SendSMTP(d.cfg, d.cfg.SMTPFrom, ev.SMTPTo, ev.SMTPSubject, ev.SMTPBody); err != nil {
				slog.ErrorContext(ctx, "notifychannels: SMTP send failed", slog.String("source", ev.LogID), slog.Any("err", err))
			}

		case "ntfy":
			if ev.NtfyURL == "" {
				slog.WarnContext(ctx, "notifychannels: ntfy URL not configured", slog.String("source", ev.LogID))
				continue
			}
			if err := d.notifier.SendNtfy(d.cfg, ev.NtfyURL, ev.NtfyTitle, ev.NtfyBody); err != nil {
				slog.ErrorContext(ctx, "notifychannels: ntfy send failed", slog.String("source", ev.LogID), slog.Any("err", err))
			}

		case "notify":
			if ev.LegacyWebhook == nil || d.cfg.NotifyURL == "" {
				continue
			}
			data, _ := json.Marshal(ev.LegacyWebhook)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.NotifyURL, bytes.NewReader(data))
			if err != nil {
				slog.ErrorContext(ctx, "notifychannels: legacy notify request build failed", slog.Any("err", err))
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				slog.ErrorContext(ctx, "notifychannels: legacy notify webhook failed", slog.Any("err", err))
				continue
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

		case "browser":
			if ev.OnBrowser != nil {
				ev.OnBrowser()
			}
			if d.pushSvc != nil && ev.Push != nil {
				go d.pushSvc.Send(ctx, d.cfg, *ev.Push)
			}

		default:
			slog.WarnContext(ctx, "notifychannels: unknown channel", slog.String("channel", ch), slog.String("source", ev.LogID))
		}
	}
}
