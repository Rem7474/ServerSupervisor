package notify

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/serversupervisor/server/internal/config"
)

func (n *notifier) SendSMTP(cfg *config.Config, from, to, subject, body string) error {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 {
		log.Printf("notify: SMTP host/port not configured")
		return fmt.Errorf("SMTP not configured")
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
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
