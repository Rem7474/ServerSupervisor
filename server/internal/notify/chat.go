package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"

	"github.com/serversupervisor/server/internal/config"
)

// Chat providers supported by SendChat. The values double as the channel
// names used in alert rules (rule.Actions.Channels).
const (
	ProviderSlack   = "slack"
	ProviderTeams   = "teams"
	ProviderDiscord = "discord"
)

// IsChatProvider reports whether name is one of the chat channel names.
func IsChatProvider(name string) bool {
	switch name {
	case ProviderSlack, ProviderTeams, ProviderDiscord:
		return true
	}
	return false
}

// ChatMessage is the provider-neutral content of a chat notification.
type ChatMessage struct {
	Title string
	Text  string
	// Color is the accent of the Discord embed (0xRRGGBB); 0 means the default.
	Color int
}

const (
	discordTitleMax = 256
	discordTextMax  = 4000
	chatTimeout     = 10 * time.Second
	defaultColor    = 0xd63939 // alert red, same hue as the UI "danger" token
)

var errInternalDestination = errors.New("destination address is not allowed")

// newChatClient builds the HTTP client used for webhook calls. Unless
// allowInternal is set (tests only), connections to loopback, private and
// link-local addresses are refused at dial time, after DNS resolution, so a
// hostname resolving to an internal address (or rebinding to one) cannot be
// used to reach internal services. Redirects are not followed.
func newChatClient(allowInternal bool) *http.Client {
	dialer := &net.Dialer{Timeout: chatTimeout}
	if !allowInternal {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			addr, err := netip.ParseAddr(host)
			if err != nil || config.IsInternalAddr(addr) {
				return errInternalDestination
			}
			return nil
		}
	}
	return &http.Client{
		Timeout:   chatTimeout,
		Transport: &http.Transport{DialContext: dialer.DialContext, Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// chatPayload renders msg in the incoming-webhook format of provider.
func chatPayload(provider string, msg ChatMessage) (any, error) {
	switch provider {
	case ProviderSlack:
		text := msg.Text
		if msg.Title != "" {
			text = "*" + msg.Title + "*\n" + msg.Text
		}
		return map[string]any{
			"text": text,
			"blocks": []any{
				map[string]any{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": text}},
			},
		}, nil
	case ProviderTeams:
		body := []any{}
		if msg.Title != "" {
			body = append(body, map[string]any{"type": "TextBlock", "text": msg.Title, "weight": "Bolder", "size": "Medium", "wrap": true})
		}
		body = append(body, map[string]any{"type": "TextBlock", "text": msg.Text, "wrap": true})
		return map[string]any{
			"type": "message",
			"attachments": []any{
				map[string]any{
					"contentType": "application/vnd.microsoft.card.adaptive",
					"content": map[string]any{
						"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
						"type":    "AdaptiveCard",
						"version": "1.4",
						"body":    body,
					},
				},
			},
		}, nil
	case ProviderDiscord:
		color := msg.Color
		if color == 0 {
			color = defaultColor
		}
		embed := map[string]any{
			"description": truncateRunes(msg.Text, discordTextMax),
			"color":       color,
		}
		if msg.Title != "" {
			embed["title"] = truncateRunes(msg.Title, discordTitleMax)
		}
		return map[string]any{"embeds": []any{embed}}, nil
	}
	return nil, fmt.Errorf("unknown chat provider %q", provider)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func (n *notifier) SendChat(provider, webhookURL string, msg ChatMessage) error {
	if webhookURL == "" {
		return fmt.Errorf("%s: webhook URL is empty", provider)
	}
	if err := config.ValidateChatWebhookURL(webhookURL); err != nil {
		return fmt.Errorf("%s: %w", provider, err)
	}
	payload, err := chatPayload(provider, msg)
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: invalid webhook request", provider)
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.chatClient
	if client == nil {
		client = defaultChatClient
	}
	resp, err := client.Do(req)
	if err != nil {
		// The webhook URL is a secret and *url.Error embeds it: keep only the cause.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("%s: request failed: %w", provider, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: webhook returned status %d", provider, resp.StatusCode)
	}
	return nil
}

var defaultChatClient = newChatClient(false)
