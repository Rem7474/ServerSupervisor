// Package settings is the application/service layer for runtime configuration. It
// owns the settings snapshot assembly, persistence (+ in-memory config reload),
// the SMTP/ntfy connectivity diagnostics and the manual retention cleanups behind
// a Repository port and the live *config.Config. HTTP authz stays in the handler.
package settings

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally; its
// GetAllSettings also satisfies config.DBSettingsLoader for the in-memory reload.
type Repository interface {
	GetAllSettings(ctx context.Context) (map[string]string, error)
	SetSetting(ctx context.Context, key, value string) error
	UpdateMetricsRetentionPolicy(ctx context.Context, days int) error
	CleanupTrackerTagDigests(ctx context.Context, keepPerTracker int) (int64, error)
	CleanOldAuditLogs(ctx context.Context, retentionDays int) (int64, error)
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
	CountAuditLogs(ctx context.Context) (int64, error)
	CountMetrics(ctx context.Context) (int64, error)
	CountHosts(ctx context.Context) (int64, error)
	Ping() error
}

// Service holds the settings use-cases.
type Service struct {
	repo          Repository
	cfg           *config.Config
	latestVersion func() string
}

func NewService(repo Repository, cfg *config.Config, latestVersion func() string) *Service {
	return &Service{repo: repo, cfg: cfg, latestVersion: latestVersion}
}

// Snapshot returns the current configuration and database status.
func (s *Service) Snapshot(ctx context.Context) map[string]any {
	c := s.cfg
	return map[string]any{
		"settings": map[string]any{
			"baseUrl":              c.BaseURL,
			"dbHost":               c.DBHost,
			"dbPort":               c.DBPort,
			"tlsEnabled":           c.TLSEnabled,
			"metricsRetentionDays": c.MetricsRetentionDays,
			"auditRetentionDays":   c.AuditRetentionDays,
			"smtpConfigured":       c.SMTPHost != "",
			"smtpHost":             c.SMTPHost,
			"smtpPort":             c.SMTPPort,
			"smtpUser":             c.SMTPUser,
			"smtpPass":             c.SMTPPass,
			"smtpFrom":             c.SMTPFrom,
			"smtpTo":               c.SMTPTo,
			"smtpTls":              c.SMTPTLS,
			"ntfyUrl":              c.NotifyURL,
			"githubToken":          c.GitHubToken,
			"latestAgentVersion":   s.latestVersion(),
		},
		"dbStatus": s.databaseStatus(ctx),
	}
}

func (s *Service) databaseStatus(ctx context.Context) map[string]any {
	connected := s.repo.Ping() == nil
	var auditLogCount, metricsCount, hostsCount int64
	if connected {
		auditLogCount, _ = s.repo.CountAuditLogs(ctx)
		metricsCount, _ = s.repo.CountMetrics(ctx)
		hostsCount, _ = s.repo.CountHosts(ctx)
	}
	return map[string]any{
		"connected":     connected,
		"auditLogCount": auditLogCount,
		"metricsCount":  metricsCount,
		"hostsCount":    hostsCount,
	}
}

// Update persists configuration changes and applies them to the in-memory config.
func (s *Service) Update(ctx context.Context, req models.SettingsUpdateRequest, username, clientIP string) {
	save := func(key, value string) {
		_ = s.repo.SetSetting(ctx, key, value)
	}
	save("smtp_host", req.SMTPHost)
	save("smtp_user", req.SMTPUser)
	save("smtp_pass", req.SMTPPass)
	save("smtp_from", req.SMTPFrom)
	save("smtp_to", req.SMTPTo)
	save("ntfy_url", req.NtfyURL)
	save("github_token", req.GitHubToken)

	if req.SMTPPort > 0 {
		save("smtp_port", strconv.Itoa(req.SMTPPort))
	}
	if req.SMTPTLS != nil {
		if *req.SMTPTLS {
			save("smtp_tls", "true")
		} else {
			save("smtp_tls", "false")
		}
	}
	if req.MetricsRetentionDays > 0 {
		save("metrics_retention_days", strconv.Itoa(req.MetricsRetentionDays))
		_ = s.repo.UpdateMetricsRetentionPolicy(ctx, req.MetricsRetentionDays)
	}
	if req.AuditRetentionDays > 0 {
		save("audit_retention_days", strconv.Itoa(req.AuditRetentionDays))
	}

	s.cfg.OverrideFromDB(s.repo)
	_, _ = s.repo.CreateAuditLog(ctx, username, "update_settings", "", clientIP, "Settings updated via UI", "success")
}

// TestSMTP performs a full SMTP connectivity / TLS / auth / envelope check and
// returns a success message. Failures carry the diagnostic verbatim.
func (s *Service) TestSMTP(_ context.Context) (string, error) {
	c := s.cfg
	if c.SMTPHost == "" {
		return "", apperr.Validation("SMTP not configured")
	}
	addr := fmt.Sprintf("%s:%d", c.SMTPHost, c.SMTPPort)
	tlsConfig := &tls.Config{ServerName: c.SMTPHost}

	var client *smtp.Client
	var err error
	if c.SMTPPort == 465 {
		conn, tlsErr := tls.Dial("tcp", addr, tlsConfig)
		if tlsErr != nil {
			return "", apperr.Failed(fmt.Sprintf("SMTPS connection failed: %v", tlsErr))
		}
		client, err = smtp.NewClient(conn, c.SMTPHost)
	} else {
		client, err = smtp.Dial(addr)
		if err == nil && c.SMTPTLS {
			if err = client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				return "", apperr.Failed(fmt.Sprintf("STARTTLS failed: %v", err))
			}
		}
	}
	if err != nil {
		return "", apperr.Failed(fmt.Sprintf("SMTP connection failed: %v", err))
	}
	defer func() { _ = client.Close() }()

	if c.SMTPUser != "" && c.SMTPPass != "" {
		auth := smtp.PlainAuth("", c.SMTPUser, c.SMTPPass, c.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return "", apperr.Failed(fmt.Sprintf("SMTP auth failed: %v", err))
		}
	}

	if c.SMTPFrom != "" && c.SMTPTo != "" {
		if err := client.Mail(c.SMTPFrom); err != nil {
			return "", apperr.Failed(fmt.Sprintf("SMTP MAIL FROM rejected: %v", err))
		}
		if err := client.Rcpt(c.SMTPTo); err != nil {
			return "", apperr.Failed(fmt.Sprintf("SMTP RCPT TO rejected: %v", err))
		}
		if wc, err := client.Data(); err == nil {
			_, _ = fmt.Fprintf(wc, "From: %s\r\nTo: %s\r\nSubject: ServerSupervisor - Test SMTP\r\n\r\nConfiguration SMTP valide.\r\n", c.SMTPFrom, c.SMTPTo)
			_ = wc.Close()
		}
		return fmt.Sprintf("Email test sent to %s", c.SMTPTo), nil
	}

	_ = client.Quit()
	return "SMTP connection and auth successful", nil
}

// TestNtfy sends a test notification to the configured ntfy topic.
func (s *Service) TestNtfy(_ context.Context) (string, error) {
	notifyURL := s.cfg.NotifyURL
	if notifyURL == "" {
		return "", apperr.Validation("ntfy.sh URL not configured")
	}
	u, err := url.Parse(notifyURL)
	if err != nil {
		return "", apperr.Validation("Invalid ntfy.sh URL")
	}
	topic := strings.TrimPrefix(u.Path, "/")
	if topic == "" {
		return "", apperr.Validation("ntfy.sh URL must include topic")
	}
	testURL := u.Scheme + "://" + u.Host + "/" + topic
	resp, err := http.Post(testURL, "text/plain", strings.NewReader("Test notification from ServerSupervisor"))
	if err != nil {
		return "", apperr.Failed(fmt.Sprintf("Failed to send notification: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", apperr.Failed(fmt.Sprintf("ntfy.sh returned status %d", resp.StatusCode))
	}
	return "Test notification sent successfully", nil
}

// CleanupMetrics reapplies the retention policy and trims tracker tag digests.
func (s *Service) CleanupMetrics(ctx context.Context, username, clientIP string) (int64, string, error) {
	if err := s.repo.UpdateMetricsRetentionPolicy(ctx, s.cfg.MetricsRetentionDays); err != nil {
		return 0, "", apperr.Failed(fmt.Sprintf("Failed to update retention policy: %v", err))
	}
	deletedDigests, _ := s.repo.CleanupTrackerTagDigests(ctx, 100)
	message := fmt.Sprintf("Politique de rétention mise à jour (%d jours). TimescaleDB appliquera le nettoyage automatiquement. %d anciennes entrées de suivi supprimées.", s.cfg.MetricsRetentionDays, deletedDigests)
	_, _ = s.repo.CreateAuditLog(ctx, username, "cleanup_metrics", "", clientIP, message, "success")
	return deletedDigests, message, nil
}

// CleanupAuditLogs deletes audit logs older than the retention window.
func (s *Service) CleanupAuditLogs(ctx context.Context, username, clientIP string) (int64, string, error) {
	deleted, err := s.repo.CleanOldAuditLogs(ctx, s.cfg.AuditRetentionDays)
	if err != nil {
		return 0, "", apperr.Failed(fmt.Sprintf("Cleanup failed: %v", err))
	}
	message := fmt.Sprintf("Deleted %d old audit log records", deleted)
	_, _ = s.repo.CreateAuditLog(ctx, username, "cleanup_audit_logs", "", clientIP, message, "success")
	return deleted, message, nil
}
