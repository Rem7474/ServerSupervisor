package notify

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/serversupervisor/server/internal/config"
)

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
		log.Printf("notify: SMTP host/port not configured")
		return fmt.Errorf("SMTP not configured")
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
		log.Printf("notify: SMTP dial failed: %v", err)
		return err
	}
	defer func() { _ = c.Close() }()

	if cfg.SMTPTLS {
		if err := c.StartTLS(&tls.Config{ServerName: cfg.SMTPHost}); err != nil {
			log.Printf("notify: SMTP StartTLS failed: %v", err)
			return err
		}
	}
	if cfg.SMTPUser != "" {
		if err := c.Auth(auth); err != nil {
			log.Printf("notify: SMTP auth failed: %v", err)
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		log.Printf("notify: SMTP MAIL FROM failed: %v", err)
		return err
	}
	if err := c.Rcpt(to); err != nil {
		log.Printf("notify: SMTP RCPT TO failed: %v", err)
		return err
	}
	w, err := c.Data()
	if err != nil {
		log.Printf("notify: SMTP DATA failed: %v", err)
		return err
	}
	_, _ = w.Write([]byte(msg))
	return w.Close()
}
