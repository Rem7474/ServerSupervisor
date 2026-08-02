package alerts

import (
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

func TestNtfyTopicURL(t *testing.T) {
	tests := []struct {
		name  string
		base  string
		topic string
		want  string
	}{
		{
			name:  "no topic returns base unchanged",
			base:  "https://ntfy.example.lan/default-topic",
			topic: "",
			want:  "https://ntfy.example.lan/default-topic",
		},
		{
			name:  "topic override rebuilds path on self-hosted server",
			base:  "https://ntfy.example.lan/default-topic",
			topic: "critical-alerts",
			want:  "https://ntfy.example.lan/critical-alerts",
		},
		{
			name:  "topic override on self-hosted server with no path configured",
			base:  "https://ntfy.example.lan",
			topic: "critical-alerts",
			want:  "https://ntfy.example.lan/critical-alerts",
		},
		{
			name:  "no base configured falls back to public ntfy.sh",
			base:  "",
			topic: "critical-alerts",
			want:  "https://ntfy.sh/critical-alerts",
		},
		{
			name:  "unparseable base falls back to public ntfy.sh",
			base:  "not a url",
			topic: "critical-alerts",
			want:  "https://ntfy.sh/critical-alerts",
		},
		{
			name:  "no base and no topic yields empty URL (channel gets skipped downstream)",
			base:  "",
			topic: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ntfyTopicURL(tt.base, tt.topic); got != tt.want {
				t.Errorf("ntfyTopicURL(%q, %q) = %q, want %q", tt.base, tt.topic, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{seconds: 30, want: "30s"},
		{seconds: 90, want: "1 min"},
		{seconds: 300, want: "5 min"},
		{seconds: 3600, want: "1h"},
		{seconds: 7200, want: "2h"},
		{seconds: 86400, want: "1j"},
		{seconds: 90000, want: "1j1h"},
		{seconds: 183600, want: "2j3h"},
	}

	for _, tt := range tests {
		if got := formatDuration(tt.seconds); got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestResolvedEvent(t *testing.T) {
	rule := models.AlertRule{ID: 3, Metric: "cpu"}
	host := models.Host{ID: "host-1", Name: "srv-01"}
	inc := models.AlertIncident{
		ID:          42,
		Value:       91.5,
		TriggeredAt: time.Now().Add(-90 * time.Second),
	}

	ev := resolvedEvent(rule, host, inc)

	if len(ev.Channels) != 1 || ev.Channels[0] != "browser" {
		t.Errorf("resolvedEvent Channels = %v, want [browser]", ev.Channels)
	}
	if ev.Push == nil {
		t.Fatal("resolvedEvent Push = nil, want non-nil")
	}
	want := "srv-01 — revenu à la normale (91.50%, durée 1 min)"
	if ev.Push.Body != want {
		t.Errorf("resolvedEvent Push.Body = %q, want %q", ev.Push.Body, want)
	}
	if ev.Push.Status != "resolved" {
		t.Errorf("resolvedEvent Push.Status = %q, want %q", ev.Push.Status, "resolved")
	}
}
