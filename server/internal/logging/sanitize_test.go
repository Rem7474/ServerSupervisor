package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "clean", in: "host-01.example.com", want: "host-01.example.com"},
		{name: "lf", in: "a\nb\nc", want: "abc"},
		{name: "cr", in: "a\rb\rc", want: "abc"},
		{name: "crlf_injection", in: "alice\r\nlevel=INFO msg=forged", want: "alicelevel=INFO msg=forged"},
		{name: "only_newlines", in: "\r\n\n\r", want: ""},
		{name: "preserves_unicode_and_tabs", in: "héllo\tworld\n!", want: "héllo\tworld!"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sanitize(tc.in); got != tc.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizePreventsLogLineInjection(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf)

	malicious := "bob\n{\"time\":\"2026-01-01\",\"level\":\"INFO\",\"msg\":\"forged\"}\r\n"
	logger.Warn("auth failed", slog.String("user", Sanitize(malicious)))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d: %q", len(lines), buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if got := rec["user"]; got != `bob{"time":"2026-01-01","level":"INFO","msg":"forged"}` {
		t.Errorf("unexpected sanitized user field: %v", got)
	}
}
