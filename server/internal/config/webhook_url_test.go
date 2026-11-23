package config

import "testing"

func TestValidateChatWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty is allowed", "", false},
		{"slack https", "https://hooks.slack.com/services/T000/B000/XXXX", false},
		{"https with port", "https://example.com:8443/hook", false},
		{"http rejected", "http://hooks.slack.com/services/x", true},
		{"no scheme", "hooks.slack.com/services/x", true},
		{"credentials rejected", "https://user:pw@hooks.slack.com/x", true},
		{"localhost rejected", "https://localhost/hook", true},
		{"sub localhost rejected", "https://foo.localhost/hook", true},
		{"loopback literal", "https://127.0.0.1/hook", true},
		{"private literal", "https://10.1.2.3/hook", true},
		{"link-local metadata", "https://169.254.169.254/latest", true},
		{"ipv6 loopback", "https://[::1]/hook", true},
		{"ipv4-mapped ipv6 private", "https://[::ffff:192.168.0.1]/hook", true},
		{"public literal ok", "https://8.8.8.8/hook", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChatWebhookURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateChatWebhookURL(%q) err = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
