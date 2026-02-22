package api

import (
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
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
	// Get database status
	dbStatus := h.getDatabaseStatus()

	response := gin.H{
		"settings": gin.H{
			"baseUrl":              h.cfg.BaseURL,
			"dbHost":               h.cfg.DBHost,
			"dbPort":               h.cfg.DBPort,
			"tlsEnabled":           h.cfg.TLSEnabled,
			"metricsRetentionDays": h.cfg.MetricsRetentionDays,
			"smtpConfigured":       h.cfg.SMTPHost != "",
			"smtpHost":             h.cfg.SMTPHost,
			"smtpPort":             h.cfg.SMTPPort,
			"ntfyUrl":              h.cfg.NotifyURL,
		},
		"dbStatus": dbStatus,
	}

	c.JSON(http.StatusOK, response)
}

// TestSmtp tests SMTP connectivity
func (h *SettingsHandler) TestSmtp(c *gin.Context) {
	if h.cfg.SMTPHost == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP not configured"})
		return
	}

	// Test SMTP connection
	addr := fmt.Sprintf("%s:%d", h.cfg.SMTPHost, h.cfg.SMTPPort)
	client, err := smtp.Dial(addr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP connection failed: %v", err)})
		return
	}
	defer client.Close()

	// If auth is required
	if h.cfg.SMTPUser != "" && h.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", h.cfg.SMTPUser, h.cfg.SMTPPass, h.cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP auth failed: %v", err)})
			return
		}
	}

	// Send QUIT command
	if err := client.Quit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("SMTP disconnection failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "SMTP connection successful"})
}

// TestNtfy sends a test notification to ntfy.sh
func (h *SettingsHandler) TestNtfy(c *gin.Context) {
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
	user := c.GetString("username")
	log.Printf("User %s triggered manual audit logs cleanup", user)

	deleted, err := h.db.CleanOldAuditLogs(90)
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
	auditLogCount, _ := h.db.CountAuditLogs()
	metricsCount, _ := h.db.CountMetrics()
	hostsCount, _ := h.db.CountHosts()

	return gin.H{
		"connected":     true,
		"auditLogCount": auditLogCount,
		"metricsCount":  metricsCount,
		"hostsCount":    hostsCount,
	}
}
