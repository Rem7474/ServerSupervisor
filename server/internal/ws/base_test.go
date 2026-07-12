package ws

import "testing"

func TestIsAllowedOrigin(t *testing.T) {
	const baseURL = "https://supervisor.example.com"
	extra := []string{"https://backup.example.com"}

	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"empty origin (non-browser client)", "", true},
		{"localhost", "http://localhost:5173", true},
		{"127.0.0.1", "http://127.0.0.1:5173", true},
		{"ipv6 loopback", "http://[::1]:5173", true},
		{"matches base URL exactly", "https://supervisor.example.com", true},
		{"matches base host, scheme mismatch still allowed", "http://supervisor.example.com", true},
		{"matches extra origin exactly", "https://backup.example.com", true},
		{"matches extra host, scheme mismatch still allowed", "http://backup.example.com", true},
		{"unrelated origin rejected", "https://evil.example.com", false},
		{"malformed origin rejected", "://not a url", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isAllowedOrigin(tc.origin, baseURL, extra)
			if got != tc.want {
				t.Errorf("isAllowedOrigin(%q, %q, %v) = %v, want %v", tc.origin, baseURL, extra, got, tc.want)
			}
		})
	}
}

func TestSnapshotChanged(t *testing.T) {
	var lastHash string

	if !snapshotChanged(map[string]int{"a": 1}, &lastHash) {
		t.Error("first call should always report a change")
	}
	firstHash := lastHash
	if firstHash == "" {
		t.Error("expected lastHash to be set after the first call")
	}

	if snapshotChanged(map[string]int{"a": 1}, &lastHash) {
		t.Error("identical payload should not report a change")
	}
	if lastHash != firstHash {
		t.Error("lastHash should not change when the payload is identical")
	}

	if !snapshotChanged(map[string]int{"a": 2}, &lastHash) {
		t.Error("different payload should report a change")
	}
	if lastHash == firstHash {
		t.Error("lastHash should update when the payload changes")
	}
}
