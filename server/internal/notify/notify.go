package notify

import (
	"github.com/serversupervisor/server/internal/config"
)

// Notifier defines the interface for sending notifications
type Notifier interface {
	SendSMTP(cfg *config.Config, from, to, subject, body string) error
	// SendNtfy posts msg to the given ntfy URL (e.g. "https://ntfy.sh/my-topic").
	SendNtfy(cfg *config.Config, url, title, msg string) error
	// Browser notifications are handled separately via NotificationHub
}

// New creates a new Notifier instance
func New() Notifier {
	return &notifier{}
}

type notifier struct{}
