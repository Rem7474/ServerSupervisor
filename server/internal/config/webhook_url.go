package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// ValidateChatWebhookURL checks the incoming-webhook URL of a chat channel
// (Slack, Teams, Discord). An empty value is valid (channel not configured).
// The URL must be https, carry no credentials in the authority and not name a
// loopback/private/link-local address or "localhost" literally. This is the
// static half of the SSRF guard; the notifier additionally refuses to dial
// such addresses when a hostname resolves to one.
func ValidateChatWebhookURL(val string) error {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid webhook URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use https")
	}
	if u.User != nil {
		return fmt.Errorf("webhook URL must not embed credentials")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("webhook URL must not target a local address")
	}
	if addr, err := netip.ParseAddr(host); err == nil && IsInternalAddr(addr) {
		return fmt.Errorf("webhook URL must not target a private or local address")
	}
	return nil
}

// IsInternalAddr reports whether addr is loopback, private, link-local,
// multicast or unspecified — destinations an outbound webhook must never reach.
func IsInternalAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified()
}
