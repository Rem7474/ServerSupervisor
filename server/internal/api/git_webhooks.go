package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

var validWebhookProviders = map[string]bool{
	"github": true, "gitlab": true, "gitea": true, "forgejo": true, "custom": true,
}

type GitWebhookHandler struct {
	db       *database.DB
	cfg      *config.Config
	notifHub *NotificationHub
}

func NewGitWebhookHandler(db *database.DB, cfg *config.Config, notifHub *NotificationHub) *GitWebhookHandler {
	return &GitWebhookHandler{db: db, cfg: cfg, notifHub: notifHub}
}

// ========== CRUD (authenticated, admin only) ==========

func (h *GitWebhookHandler) ListWebhooks(c *gin.Context) {
	webhooks, err := h.db.ListGitWebhooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list webhooks"})
		return
	}
	if webhooks == nil {
		webhooks = []models.GitWebhook{}
	}
	c.JSON(http.StatusOK, gin.H{"webhooks": webhooks})
}

func (h *GitWebhookHandler) CreateWebhook(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}

	var req models.GitWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validWebhookProviders[req.Provider] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider; must be github, gitlab, gitea, forgejo, or custom"})
		return
	}
	if req.HostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id is required"})
		return
	}
	if req.CustomTaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "custom_task_id is required"})
		return
	}
	if req.EventFilter == "" {
		req.EventFilter = "push"
	}
	if req.NotifyChannels == nil {
		req.NotifyChannels = []string{}
	}

	created, err := h.db.CreateGitWebhook(req)
	if err != nil {
		log.Printf("CreateWebhook: db error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create webhook"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"webhook": created})
}

func (h *GitWebhookHandler) GetWebhook(c *gin.Context) {
	id := c.Param("id")
	wh, err := h.db.GetGitWebhookByID(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get webhook"})
		return
	}
	execs, _ := h.db.ListWebhookExecutions(id, 20)
	c.JSON(http.StatusOK, gin.H{"webhook": wh, "executions": execs})
}

func (h *GitWebhookHandler) UpdateWebhook(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")

	var req models.GitWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if !validWebhookProviders[req.Provider] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
		return
	}
	if req.HostID == "" || req.CustomTaskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id and custom_task_id are required"})
		return
	}

	if err := h.db.UpdateGitWebhook(id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *GitWebhookHandler) DeleteWebhook(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")
	if err := h.db.DeleteGitWebhook(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete webhook"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *GitWebhookHandler) RegenerateSecret(c *gin.Context) {
	role := c.GetString("role")
	if role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required"})
		return
	}
	id := c.Param("id")
	secret, err := h.db.RegenerateWebhookSecret(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to regenerate secret"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"secret": secret})
}

func (h *GitWebhookHandler) GetWebhookExecutions(c *gin.Context) {
	id := c.Param("id")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	execs, err := h.db.ListWebhookExecutions(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list executions"})
		return
	}
	if execs == nil {
		execs = []models.GitWebhookExecution{}
	}
	c.JSON(http.StatusOK, gin.H{"executions": execs})
}

// ========== Public receiver ==========

// ReceiveWebhook is the public endpoint called by Git providers.
// It verifies the HMAC signature, applies filters, then dispatches a custom command to the agent.
func (h *GitWebhookHandler) ReceiveWebhook(c *gin.Context) {
	id := c.Param("id")

	// Read body upfront (needed for HMAC computation)
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 5<<20)) // 5MB limit
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	wh, err := h.db.GetGitWebhookForReceive(id)
	if err == sql.ErrNoRows || (err == nil && !wh.Enabled) {
		c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Verify signature
	if !verifyWebhookSignature(wh.Provider, wh.Secret, body, c.Request.Header) {
		log.Printf("Webhook %s: signature verification failed (IP: %s)", id, c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	// Parse payload
	eventHeader := c.GetHeader("X-GitHub-Event")
	if eventHeader == "" {
		eventHeader = c.GetHeader("X-Gitea-Event")
	}
	if eventHeader == "" {
		eventHeader = c.GetHeader("X-Forgejo-Event")
	}
	if eventHeader == "" {
		eventHeader = c.GetHeader("X-Gitlab-Event")
	}
	if eventHeader == "" {
		eventHeader = "push"
	}

	parsed, err := parseGitPayload(wh.Provider, body, eventHeader)
	if err != nil {
		log.Printf("Webhook %s: failed to parse payload: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse git payload"})
		return
	}
	parsed.EventType = normalizeEvent(eventHeader)

	// Apply event filter
	if wh.EventFilter != "" && wh.EventFilter != "push" {
		if parsed.EventType != wh.EventFilter {
			c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "event_filter"})
			return
		}
	}

	// Apply repo filter
	if !matchFilter(wh.RepoFilter, parsed.RepoName) {
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "repo_filter"})
		return
	}

	// Apply branch filter
	if !matchFilter(wh.BranchFilter, parsed.Branch) {
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "branch_filter"})
		return
	}

	// Check for already-running execution (skip to avoid concurrent conflicts)
	running, _ := h.db.GetRunningExecutionForWebhook(id)
	if running {
		exec := models.GitWebhookExecution{
			WebhookID:     id,
			Provider:      wh.Provider,
			RepoName:      parsed.RepoName,
			Branch:        parsed.Branch,
			CommitSHA:     parsed.CommitSHA,
			CommitMessage: parsed.CommitMessage,
			Pusher:        parsed.Pusher,
			Status:        "skipped",
		}
		created, _ := h.db.CreateWebhookExecution(exec)
		now := time.Now()
		if created != nil {
			_ = h.db.UpdateWebhookExecutionStatus(created.ID, "skipped", &now)
		}
		c.JSON(http.StatusOK, gin.H{"status": "skipped", "reason": "already_running"})
		return
	}

	// Create execution record
	exec := models.GitWebhookExecution{
		WebhookID:     id,
		Provider:      wh.Provider,
		RepoName:      parsed.RepoName,
		Branch:        parsed.Branch,
		CommitSHA:     parsed.CommitSHA,
		CommitMessage: parsed.CommitMessage,
		Pusher:        parsed.Pusher,
		Status:        "pending",
	}
	createdExec, err := h.db.CreateWebhookExecution(exec)
	if err != nil {
		log.Printf("Webhook %s: failed to create execution record: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create execution"})
		return
	}

	// Build env vars payload for the agent
	envVars := map[string]string{
		"SS_REPO_NAME":      parsed.RepoName,
		"SS_BRANCH":         parsed.Branch,
		"SS_COMMIT_SHA":     parsed.CommitSHA,
		"SS_COMMIT_MESSAGE": parsed.CommitMessage,
		"SS_PUSHER":         parsed.Pusher,
		"SS_WEBHOOK_NAME":   wh.Name,
		"SS_EVENT_TYPE":     parsed.EventType,
	}
	envPayload, _ := json.Marshal(map[string]interface{}{"env": envVars})

	// Create audit log
	username := fmt.Sprintf("webhook:%s", wh.Name)
	details := fmt.Sprintf(`{"webhook_id":%q,"repo":%q,"branch":%q,"commit":%q}`,
		id, parsed.RepoName, parsed.Branch, parsed.CommitSHA)
	auditID, auditErr := h.db.CreateAuditLog(username, "webhook_trigger", wh.HostID, c.ClientIP(), details, "pending")
	var auditIDPtr *int64
	if auditErr == nil {
		auditIDPtr = &auditID
	}

	// Create remote command
	triggeredBy := fmt.Sprintf("webhook:%s", wh.Name)
	cmd, err := h.db.CreateRemoteCommand(
		wh.HostID, "custom", "run", wh.CustomTaskID, string(envPayload), triggeredBy, auditIDPtr,
	)
	if err != nil {
		log.Printf("Webhook %s: failed to create remote command: %v", id, err)
		if auditIDPtr != nil {
			_ = h.db.UpdateAuditLogStatus(*auditIDPtr, "failed", err.Error())
		}
		_ = h.db.UpdateWebhookExecutionStatus(createdExec.ID, "failed", ptrNow())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to dispatch command"})
		return
	}

	// Link execution to command
	_ = h.db.UpdateWebhookExecutionCommandID(createdExec.ID, cmd.ID)
	_ = h.db.UpdateGitWebhookLastTriggered(id)

	log.Printf("Webhook %s: dispatched command %s → host %s task %s (repo=%s branch=%s)",
		id, cmd.ID, wh.HostID, wh.CustomTaskID, parsed.RepoName, parsed.Branch)

	c.JSON(http.StatusOK, gin.H{
		"status":       "dispatched",
		"execution_id": createdExec.ID,
		"command_id":   cmd.ID,
	})
}

func ptrNow() *time.Time {
	t := time.Now()
	return &t
}

// ========== Notification dispatch (called from agent result handler) ==========

// NotifyWebhookExecutionComplete updates the execution status and sends notifications
// when a webhook-triggered command completes or fails. Safe to call in a goroutine.
func (h *GitWebhookHandler) NotifyWebhookExecutionComplete(commandID, status string) {
	webhookID, notifyOnSuccess, notifyOnFailure, channels, err := h.db.UpdateWebhookExecutionByCommandID(commandID, status)
	if err != nil {
		// Not a webhook-triggered command — nothing to do
		return
	}

	shouldNotify := (status == "completed" && notifyOnSuccess) ||
		(status == "failed" && notifyOnFailure)
	if !shouldNotify || len(channels) == 0 {
		return
	}

	// Get webhook info for the notification message
	wh, err := h.db.GetGitWebhookForReceive(webhookID)
	if err != nil {
		return
	}

	emoji := "✅"
	if status == "failed" {
		emoji = "❌"
	}
	subject := fmt.Sprintf("[ServerSupervisor] Webhook %s %s %s", wh.Name, emoji, status)
	msg := fmt.Sprintf("Webhook '%s' execution %s on host %s (task: %s)",
		wh.Name, status, wh.HostID, wh.CustomTaskID)

	n := notify.New()
	for _, ch := range channels {
		switch ch {
		case "smtp":
			to := h.cfg.SMTPTo
			if to == "" || h.cfg.SMTPFrom == "" {
				continue
			}
			if err := n.SendSMTP(h.cfg, h.cfg.SMTPFrom, to, subject, msg); err != nil {
				log.Printf("Webhook SMTP send: %v", err)
			}

		case "ntfy":
			ntfyURL := h.cfg.NotifyURL
			if ntfyURL == "" {
				continue
			}
			if err := n.SendNtfy(h.cfg, ntfyURL, subject, msg); err != nil {
				log.Printf("Webhook notify ntfy: %v", err)
			}

		case "browser":
			if h.notifHub == nil {
				continue
			}
			h.notifHub.Broadcast(map[string]interface{}{
				"type": "webhook_execution",
				"notification": map[string]interface{}{
					"webhook_id":   webhookID,
					"webhook_name": wh.Name,
					"status":       status,
					"triggered_at": time.Now().UTC(),
				},
			})
		}
	}
}

// ========== Signature verification ==========

func verifyWebhookSignature(provider, secret string, body []byte, headers http.Header) bool {
	switch provider {
	case "gitlab":
		// GitLab uses a plain token (not HMAC)
		return hmac.Equal([]byte(headers.Get("X-Gitlab-Token")), []byte(secret))

	default:
		// GitHub, Gitea, Forgejo, custom: X-Hub-Signature-256: sha256=<hex>
		sigHeader := headers.Get("X-Hub-Signature-256")
		if sigHeader == "" {
			// Also check Gitea/Forgejo variant
			sigHeader = headers.Get("X-Gitea-Signature")
		}
		if sigHeader == "" {
			sigHeader = headers.Get("X-Forgejo-Signature")
		}
		if sigHeader == "" {
			return false
		}
		const prefix = "sha256="
		if !strings.HasPrefix(sigHeader, prefix) {
			return false
		}
		gotHex := strings.TrimPrefix(sigHeader, prefix)
		gotBytes, err := hex.DecodeString(gotHex)
		if err != nil {
			return false
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := mac.Sum(nil)
		return hmac.Equal(gotBytes, expected)
	}
}

// ========== Payload parsing ==========

type parsedGitPayload struct {
	RepoName      string
	Branch        string
	CommitSHA     string
	CommitMessage string
	Pusher        string
	EventType     string
}

func parseGitPayload(provider string, body []byte, eventHeader string) (*parsedGitPayload, error) {
	switch provider {
	case "gitlab":
		return parseGitLabPayload(body)
	default:
		// GitHub, Gitea, Forgejo, custom all follow the GitHub payload format
		return parseGitHubPayload(body, eventHeader)
	}
}

func parseGitHubPayload(body []byte, eventHeader string) (*parsedGitPayload, error) {
	var p struct {
		Ref  string `json:"ref"`
		After string `json:"after"`
		HeadCommit *struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"head_commit"`
		Repository *struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
		Pusher *struct {
			Name string `json:"name"`
		} `json:"pusher"`
		Sender *struct {
			Login string `json:"login"`
		} `json:"sender"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}

	result := &parsedGitPayload{}
	if p.Repository != nil {
		result.RepoName = p.Repository.FullName
	}
	result.Branch = refToBranch(p.Ref)
	result.CommitSHA = p.After
	if p.HeadCommit != nil {
		result.CommitMessage = firstLine(p.HeadCommit.Message)
		if result.CommitSHA == "" {
			result.CommitSHA = p.HeadCommit.ID
		}
	}
	if p.Pusher != nil {
		result.Pusher = p.Pusher.Name
	} else if p.Sender != nil {
		result.Pusher = p.Sender.Login
	}
	return result, nil
}

func parseGitLabPayload(body []byte) (*parsedGitPayload, error) {
	var p struct {
		Ref      string `json:"ref"`
		After    string `json:"after"`
		UserName string `json:"user_name"`
		Commits  []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"commits"`
		Project *struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}

	result := &parsedGitPayload{}
	if p.Project != nil {
		result.RepoName = p.Project.PathWithNamespace
	}
	result.Branch = refToBranch(p.Ref)
	result.CommitSHA = p.After
	result.Pusher = p.UserName
	if len(p.Commits) > 0 {
		result.CommitMessage = firstLine(p.Commits[len(p.Commits)-1].Message)
		if result.CommitSHA == "" {
			result.CommitSHA = p.Commits[len(p.Commits)-1].ID
		}
	}
	return result, nil
}

// ========== Helpers ==========

func refToBranch(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	if strings.HasPrefix(ref, "refs/tags/") {
		return strings.TrimPrefix(ref, "refs/tags/")
	}
	return ref
}

func normalizeEvent(eventHeader string) string {
	// Map provider-specific event names to our canonical set
	lower := strings.ToLower(eventHeader)
	switch {
	case lower == "push":
		return "push"
	case lower == "create", lower == "tag_push", strings.Contains(lower, "tag"):
		return "tag"
	case lower == "release", lower == "releases":
		return "release"
	default:
		return lower
	}
}

func matchFilter(filter, value string) bool {
	if filter == "" {
		return true
	}
	// Exact match or wildcard via filepath.Match
	if filter == value {
		return true
	}
	matched, _ := filepath.Match(filter, value)
	return matched
}

func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
