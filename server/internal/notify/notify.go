package notify

import (
	"net/http"

	"github.com/serversupervisor/server/internal/config"
)

// Notifier defines the interface for sending notifications
type Notifier interface {
	SendSMTP(cfg *config.Config, from, to, subject, body string) error
	// SendNtfy posts msg to the given ntfy URL (e.g. "https://ntfy.sh/my-topic").
	SendNtfy(cfg *config.Config, url, title, msg string) error
	// SendChat posts msg to a Slack, Teams or Discord incoming webhook.
	// The webhook URL is a secret and never appears in the returned error.
	SendChat(provider, webhookURL string, msg ChatMessage) error
	// Browser notifications are handled separately via NotificationHub
}

// New creates a new Notifier instance
func New() Notifier {
	return &notifier{}
}

type notifier struct {
	// chatClient overrides the SSRF-guarded default client (tests only).
	chatClient *http.Client
}
