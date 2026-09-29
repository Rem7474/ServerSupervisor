package notify

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/config"
)

func TestEnvelopeAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain address", "ops@example.com", "ops@example.com", false},
		{"display name is dropped", "Ops Team <ops@example.com>", "ops@example.com", false},
		{"CRLF smuggling is neutralised", "a@example.com\r\nRCPT TO:<evil@example.com>", "", true},
		{"CR/LF inside the local part collapses", "a\r\n@example.com", "a@example.com", false},
		{"empty", "", "", true},
		{"not an address", "not-an-address", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envelopeAddress(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("envelopeAddress(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("envelopeAddress(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// fakeSMTP accepts one connection and records the MAIL FROM / RCPT TO lines.
func fakeSMTP(t *testing.T) (port int, commands <-chan []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	out := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		write("220 fake ESMTP")
		var seen []string
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					write("250 queued")
				}
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				write("250 fake")
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				seen = append(seen, line)
				write("250 ok")
			case line == "DATA":
				inData = true
				write("354 go")
			case line == "QUIT":
				write("221 bye")
				out <- seen
				return
			default:
				write("250 ok")
			}
		}
		out <- seen
	}()
	return ln.Addr().(*net.TCPAddr).Port, out
}

func TestSendSMTP_SendsOnlyCleanEnvelopeAddresses(t *testing.T) {
	port, commands := fakeSMTP(t)
	cfg := &config.Config{SMTPHost: "127.0.0.1", SMTPPort: port}

	err := New().SendSMTP(cfg, "Alerts <alerts@example.com>", "ops@example.com", "subject\r\nBcc: x@example.com", "body")
	if err != nil {
		t.Fatalf("SendSMTP: %v", err)
	}
	got := <-commands
	want := []string{"MAIL FROM:<alerts@example.com>", "RCPT TO:<ops@example.com>"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("envelope commands = %v, want %v", got, want)
	}
}

func TestSendSMTP_RejectsInjectedRecipientBeforeConnecting(t *testing.T) {
	// Port 1 is never listening: reaching Dial would surface a connection
	// error instead of the address error asserted below.
	cfg := &config.Config{SMTPHost: "127.0.0.1", SMTPPort: 1}

	err := New().SendSMTP(cfg, "alerts@example.com", "ops@example.com\r\nRCPT TO:<evil@example.com>", "s", "b")
	if err == nil || !strings.Contains(err.Error(), "invalid email address") {
		t.Fatalf("err = %v, want an invalid email address error", err)
	}
}
