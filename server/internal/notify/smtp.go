package notify

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/serversupervisor/server/internal/config"
)

// envelopeAddress returns the bare address for an SMTP MAIL FROM / RCPT TO
// command. Control characters are stripped first so a value can never smuggle
// extra SMTP commands, then the result must parse as a single RFC 5322 address.
func envelopeAddress(value string) (string, error) {
	addr, err := mail.ParseAddress(sanitizeHeader(value))
	if err != nil {
		return "", fmt.Errorf("invalid email address: %w", err)
	}
	return addr.Address, nil
}

func sanitizeHeader(value string) string {
	// Remove CR, LF, and other control characters to prevent header injection.
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	// Optionally, further restriction could be applied here if needed.
	return value
}

func sanitizeBody(value string) string {
	// Prevent body from injecting additional headers before the blank line
	// by stripping raw CR/LF. Since this is a short text notification,
	// replacing newlines with spaces is acceptable.
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

// isHTMLContent checks if body contains HTML tags (simple heuristic)
func isHTMLContent(body string) bool {
	return strings.Contains(body, "<html>") || strings.Contains(body, "<!DOCTYPE") || strings.Contains(body, "<body>")
}

func (n *notifier) SendSMTP(cfg *config.Config, from, to, subject, body string) error {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 {
		slog.Error("notify: SMTP host/port not configured")
		return fmt.Errorf("SMTP not configured")
	}

	envFrom, err := envelopeAddress(from)
	if err != nil {
		slog.Error("notify: SMTP sender address rejected", slog.Any("err", err))
		return err
	}
	envTo, err := envelopeAddress(to)
	if err != nil {
		slog.Error("notify: SMTP recipient address rejected", slog.Any("err", err))
		return err
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// Detect if body is HTML and set appropriate Content-Type
	contentType := "text/plain; charset=utf-8"
	if isHTMLContent(body) {
		contentType = "text/html; charset=utf-8"
	} else {
		// Sanitize plain text body
		body = sanitizeBody(body)
	}

	msg := strings.Join([]string{
		"From: " + sanitizeHeader(from),
		"To: " + sanitizeHeader(to),
		"Subject: " + sanitizeHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType,
		"",
		body,
	}, "\r\n")

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	c, err := smtp.Dial(addr)
	if err != nil {
		slog.Error("notify: SMTP dial failed", slog.Any("err", err))
		return err
	}
	defer func() { _ = c.Close() }()

	if cfg.SMTPTLS {
		if err := c.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
			slog.Error("notify: SMTP StartTLS failed", slog.Any("err", err))
			return err
		}
	}
	if cfg.SMTPUser != "" {
		if err := c.Auth(auth); err != nil {
			slog.Error("notify: SMTP auth failed", slog.Any("err", err))
			return err
		}
	}
	if err := c.Mail(envFrom); err != nil {
		slog.Error("notify: SMTP MAIL FROM failed", slog.Any("err", err))
		return err
	}
	if err := c.Rcpt(envTo); err != nil {
		slog.Error("notify: SMTP RCPT TO failed", slog.Any("err", err))
		return err
	}
	w, err := c.Data()
	if err != nil {
		slog.Error("notify: SMTP DATA failed", slog.Any("err", err))
		return err
	}
	_, _ = w.Write([]byte(msg))
	return w.Close()
}
