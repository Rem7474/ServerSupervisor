package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type SettingsHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewSettingsHandler(db *database.DB, cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{db: db, cfg: cfg}
}

// GetSettings returns system configuration and database status
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	dbStatus := h.getDatabaseStatus(c.Request.Context())
	latestAgentVersion := ResolveLatestAgentVersion(h.cfg)

	response := gin.H{
		"settings": gin.H{
			"baseUrl":              h.cfg.BaseURL,
			"dbHost":               h.cfg.DBHost,
			"dbPort":               h.cfg.DBPort,
			"tlsEnabled":           h.cfg.TLSEnabled,
			"metricsRetentionDays": h.cfg.MetricsRetentionDays,
			"auditRetentionDays":   h.cfg.AuditRetentionDays,
			"smtpConfigured":       h.cfg.SMTPHost != "",
			"smtpHost":             h.cfg.SMTPHost,
			"smtpPort":             h.cfg.SMTPPort,
			"smtpUser":             h.cfg.SMTPUser,
			"smtpPass":             h.cfg.SMTPPass,
			"smtpFrom":             h.cfg.SMTPFrom,
			"smtpTo":               h.cfg.SMTPTo,
			"smtpTls":              h.cfg.SMTPTLS,
			"ntfyUrl":              h.cfg.NotifyURL,
			"githubToken":          h.cfg.GitHubToken,
			"latestAgentVersion":   latestAgentVersion,
		},
		"dbStatus": dbStatus,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateSettings persists configuration changes to the database and applies them in memory.
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req models.SettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	save := func(key, value string) {
		if err := h.db.SetSetting(c.Request.Context(), key, value); err != nil {
			slog.ErrorContext(c.Request.Context(), fmt.Sprintf("Failed to persist setting %s: %v", key, err))
		}
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
		if err := h.db.UpdateMetricsRetentionPolicy(c.Request.Context(), req.MetricsRetentionDays); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to update metrics retention policy", slog.Any("err", err))
		}
	}
	if req.AuditRetentionDays > 0 {
		save("audit_retention_days", strconv.Itoa(req.AuditRetentionDays))
	}

	// Apply persisted settings to in-memory config immediately
	h.cfg.OverrideFromDB(h.db)

	user := c.GetString("username")
	_, _ = h.db.CreateAuditLog(c.Request.Context(), user, "update_settings", "", c.ClientIP(), "Settings updated via UI", "success")

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Paramètres mis à jour"})
}

// TestSmtp tests SMTP connectivity with full TLS/auth/envelope validation
func (h *SettingsHandler) TestSmtp(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	if h.cfg.SMTPHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP not configured"})
		return
	}

	addr := fmt.Sprintf("%s:%d", h.cfg.SMTPHost, h.cfg.SMTPPort)
	tlsConfig := &tls.Config{ServerName: h.cfg.SMTPHost}

	var client *smtp.Client
	var err error

	// Port 465: SMTPS (TLS from the start)
	if h.cfg.SMTPPort == 465 {
		conn, tlsErr := tls.Dial("tcp", addr, tlsConfig)
		if tlsErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTPS connection failed: %v", tlsErr)})
			return
		}
		client, err = smtp.NewClient(conn, h.cfg.SMTPHost)
	} else {
		// Port 587 or other: plain connection then STARTTLS
		client, err = smtp.Dial(addr)
		if err == nil && h.cfg.SMTPTLS {
			if err = client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("STARTTLS failed: %v", err)})
				return
			}
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP connection failed: %v", err)})
		return
	}
	defer func() { _ = client.Close() }()

	if h.cfg.SMTPUser != "" && h.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", h.cfg.SMTPUser, h.cfg.SMTPPass, h.cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP auth failed: %v", err)})
			return
		}
	}

	// Test MAIL FROM / RCPT TO if configured
	if h.cfg.SMTPFrom != "" && h.cfg.SMTPTo != "" {
		if err := client.Mail(h.cfg.SMTPFrom); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP MAIL FROM rejected: %v", err)})
			return
		}
		if err := client.Rcpt(h.cfg.SMTPTo); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP RCPT TO rejected: %v", err)})
			return
		}
		wc, err := client.Data()
		if err == nil {
			_, _ = fmt.Fprintf(wc, "From: %s\r\nTo: %s\r\nSubject: ServerSupervisor - Test SMTP\r\n\r\nConfiguration SMTP valide.\r\n", h.cfg.SMTPFrom, h.cfg.SMTPTo)
			_ = wc.Close()
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Email test sent to %s", h.cfg.SMTPTo)})
		return
	}

	_ = client.Quit()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "SMTP connection and auth successful"})
}

// TestNtfy sends a test notification to ntfy.sh
func (h *SettingsHandler) TestNtfy(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	if h.cfg.NotifyURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ntfy.sh URL not configured"})
		return
	}

	// Parse URL to get base path
	u, err := url.Parse(h.cfg.NotifyURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ntfy.sh URL"})
		return
	}

	// Extract topic from URL
	topic := strings.TrimPrefix(u.Path, "/")
	if topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ntfy.sh URL must include topic"})
		return
	}

	// Send test message
	testURL := u.Scheme + "://" + u.Host + "/" + topic
	resp, err := http.Post(testURL, "text/plain", strings.NewReader("Test notification from ServerSupervisor"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send notification: %v", err)})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ntfy.sh returned status %d", resp.StatusCode)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Test notification sent successfully"})
}

// CleanupMetrics triggers manual cleanup of old metrics
func (h *SettingsHandler) CleanupMetrics(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	user := c.GetString("username")
	slog.InfoContext(c.Request.Context(), fmt.Sprintf("User %s triggered manual metrics cleanup", user))

	if err := h.db.UpdateMetricsRetentionPolicy(c.Request.Context(), h.cfg.MetricsRetentionDays); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update retention policy: %v", err)})
		return
	}

	// Also trim old tracker tag digests (keep last 100 per tracker).
	deletedDigests, digestErr := h.db.CleanupTrackerTagDigests(c.Request.Context(), 100)
	if digestErr != nil {
		slog.ErrorContext(c.Request.Context(), fmt.Sprintf("CleanupMetrics: failed to trim tracker tag digests: %v", digestErr))
	}

	message := fmt.Sprintf("Politique de rétention mise à jour (%d jours). TimescaleDB appliquera le nettoyage automatiquement. %d anciennes entrées de suivi supprimées.", h.cfg.MetricsRetentionDays, deletedDigests)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted_digests": deletedDigests})

	// Log the action
	_, _ = h.db.CreateAuditLog(c.Request.Context(), user, "cleanup_metrics", "", c.ClientIP(), message, "success")
}

// CleanupAuditLogs triggers manual cleanup of old audit logs
func (h *SettingsHandler) CleanupAuditLogs(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	user := c.GetString("username")
	slog.InfoContext(c.Request.Context(), fmt.Sprintf("User %s triggered manual audit logs cleanup", user))

	deleted, err := h.db.CleanOldAuditLogs(c.Request.Context(), h.cfg.AuditRetentionDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Cleanup failed: %v", err)})
		return
	}

	message := fmt.Sprintf("Deleted %d old audit log records", deleted)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted": deleted})

	// Log the action
	_, _ = h.db.CreateAuditLog(c.Request.Context(), user, "cleanup_audit_logs", "", c.ClientIP(), message, "success")
}

// getDatabaseStatus returns current database statistics
func (h *SettingsHandler) getDatabaseStatus(ctx context.Context) gin.H {
	connected := h.db.Ping() == nil

	var auditLogCount, metricsCount, hostsCount int64
	if connected {
		auditLogCount, _ = h.db.CountAuditLogs(ctx)
		metricsCount, _ = h.db.CountMetrics(ctx)
		hostsCount, _ = h.db.CountHosts(ctx)
	}

	return gin.H{
		"connected":     connected,
		"auditLogCount": auditLogCount,
		"metricsCount":  metricsCount,
		"hostsCount":    hostsCount,
	}
}
