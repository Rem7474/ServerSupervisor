package agentws

import (
	"testing"
	"time"
)

func TestToWebSocketURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"http becomes ws", "http://server.example:8080", "ws://server.example:8080/api/agent/ws", false},
		{"https becomes wss", "https://server.example", "wss://server.example/api/agent/ws", false},
		{"trailing slash is trimmed before appending the path", "http://server.example/", "ws://server.example/api/agent/ws", false},
		{"unsupported scheme errors", "ftp://server.example", "", true},
		{"unparseable url errors", "://not a url", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := toWebSocketURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("toWebSocketURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	if got := nextBackoff(minBackoff); got != minBackoff*2 {
		t.Errorf("nextBackoff(min) = %v, want %v", got, minBackoff*2)
	}
	if got := nextBackoff(maxBackoff); got != maxBackoff {
		t.Errorf("nextBackoff(max) = %v, want it capped at %v", got, maxBackoff)
	}
	if got := nextBackoff(maxBackoff/2 + time.Second); got != maxBackoff {
		t.Errorf("nextBackoff just under max should cap at max, got %v", got)
	}
}
