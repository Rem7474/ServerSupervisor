package sender

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serversupervisor/agent/internal/config"
)

func TestStreamProgressReadsCancelFlag(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"cancel requested", http.StatusOK, `{"status":"ok","cancel":true}`, true},
		{"keep going", http.StatusOK, `{"status":"ok"}`, false},
		{"older server, non-JSON body", http.StatusOK, `ok`, false},
		{"server error", http.StatusInternalServerError, `{"cancel":true}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/agent/command/stream" || r.Header.Get("X-API-Key") != "key" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&got)
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			s := New(&config.Config{ServerURL: srv.URL, APIKey: "key"})
			if cancel := s.StreamProgress(context.Background(), "cmd-9", `{"done":1}`); cancel != tt.want {
				t.Fatalf("cancel = %v, want %v", cancel, tt.want)
			}
			if got["command_id"] != "cmd-9" || got["chunk"] != `{"done":1}` {
				t.Fatalf("posted body = %v", got)
			}
		})
	}
}

func TestStreamProgressUnreachableServer(t *testing.T) {
	s := New(&config.Config{ServerURL: "http://127.0.0.1:1", APIKey: "key"})
	if s.StreamProgress(context.Background(), "cmd", "{}") {
		t.Fatal("an unreachable server must not cancel the scan")
	}
}
