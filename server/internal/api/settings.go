package api

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// LatestAgentVersion is the current agent release version.
// Update this constant when cutting a new agent release.
const LatestAgentVersion = "1.3.0"

type SettingsHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewSettingsHandler(db *database.DB, cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{db: db, cfg: cfg}
}

// GetSettings returns system configuration and database status
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	dbStatus := h.getDatabaseStatus()

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
			"latestAgentVersion":   LatestAgentVersion,
		},
		"dbStatus": dbStatus,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateSettingsRequest defines the body for PUT /settings.
type UpdateSettingsRequest struct {
	SMTPHost             string `json:"smtp_host"`
	SMTPPort             int    `json:"smtp_port"`
	SMTPUser             string `json:"smtp_user"`
	SMTPPass             string `json:"smtp_pass"`
	SMTPFrom             string `json:"smtp_from"`
	SMTPTo               string `json:"smtp_to"`
	SMTPTLS              *bool  `json:"smtp_tls"`
	NtfyURL              string `json:"ntfy_url"`
	GitHubToken          string `json:"github_token"`
	MetricsRetentionDays int    `json:"metrics_retention_days"`
	AuditRetentionDays   int    `json:"audit_retention_days"`
}

// UpdateSettings persists configuration changes to the database and applies them in memory.
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	save := func(key, value string) {
		if err := h.db.SetSetting(key, value); err != nil {
			log.Printf("Failed to persist setting %s: %v", key, err)
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
	}
	if req.AuditRetentionDays > 0 {
		save("audit_retention_days", strconv.Itoa(req.AuditRetentionDays))
	}

	// Apply persisted settings to in-memory config immediately
	h.cfg.OverrideFromDB(h.db)

	user := c.GetString("username")
	_, _ = h.db.CreateAuditLog(user, "update_settings", "", c.ClientIP(), "Settings updated via UI", "success")

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
				client.Close()
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("STARTTLS failed: %v", err)})
				return
			}
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP connection failed: %v", err)})
		return
	}
	defer client.Close()

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
			fmt.Fprintf(wc, "From: %s\r\nTo: %s\r\nSubject: ServerSupervisor - Test SMTP\r\n\r\nConfiguration SMTP valide.\r\n", h.cfg.SMTPFrom, h.cfg.SMTPTo)
			wc.Close()
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Email test sent to %s", h.cfg.SMTPTo)})
		return
	}

	client.Quit()
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
	defer resp.Body.Close()

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
	log.Printf("User %s triggered manual metrics cleanup", user)

	deleted, err := h.db.CleanOldMetrics(h.cfg.MetricsRetentionDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Cleanup failed: %v", err)})
		return
	}

	message := fmt.Sprintf("Deleted %d old metrics records", deleted)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted": deleted})

	// Log the action
	_, _ = h.db.CreateAuditLog(user, "cleanup_metrics", "", c.ClientIP(), message, "success")
}

// CleanupAuditLogs triggers manual cleanup of old audit logs
func (h *SettingsHandler) CleanupAuditLogs(c *gin.Context) {
	if c.GetString("role") != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return
	}
	user := c.GetString("username")
	log.Printf("User %s triggered manual audit logs cleanup", user)

	deleted, err := h.db.CleanOldAuditLogs(h.cfg.AuditRetentionDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Cleanup failed: %v", err)})
		return
	}

	message := fmt.Sprintf("Deleted %d old audit log records", deleted)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "deleted": deleted})

	// Log the action
	_, _ = h.db.CreateAuditLog(user, "cleanup_audit_logs", "", c.ClientIP(), message, "success")
}

// getDatabaseStatus returns current database statistics
func (h *SettingsHandler) getDatabaseStatus() gin.H {
	connected := h.db.Ping() == nil

	var auditLogCount, metricsCount, hostsCount int64
	if connected {
		auditLogCount, _ = h.db.CountAuditLogs()
		metricsCount, _ = h.db.CountMetrics()
		hostsCount, _ = h.db.CountHosts()
	}

	return gin.H{
		"connected":     connected,
		"auditLogCount": auditLogCount,
		"metricsCount":  metricsCount,
		"hostsCount":    hostsCount,
	}
}
