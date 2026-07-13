package alerts

import "testing"

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

func TestFormatCooldownDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{seconds: 30, want: "30s"},
		{seconds: 90, want: "1 min"},
		{seconds: 300, want: "5 min"},
		{seconds: 3600, want: "1h"},
		{seconds: 7200, want: "2h"},
	}

	for _, tt := range tests {
		if got := formatCooldownDuration(tt.seconds); got != tt.want {
			t.Errorf("formatCooldownDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}
