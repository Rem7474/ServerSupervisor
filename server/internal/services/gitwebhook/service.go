// Package gitwebhook is the application/service layer for Git webhooks: the
// admin CRUD, the public receiver (HMAC verification, multi-provider payload
// parsing, event/repo/branch filtering, agent-command dispatch) and the
// completion notifications. It sits behind a Repository + Dispatcher port; cfg +
// the notification hub are held for the completion fan-out.
package gitwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/ws"
)

var validWebhookProviders = map[string]bool{
	"github": true, "gitlab": true, "gitea": true, "forgejo": true, "custom": true,
}

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListGitWebhooks(ctx context.Context) ([]models.GitWebhook, error)
	CreateGitWebhook(ctx context.Context, w models.GitWebhook) (*models.GitWebhook, error)
	GetGitWebhookByID(ctx context.Context, id string) (*models.GitWebhook, error)
	GetGitWebhookForReceive(ctx context.Context, id string) (*models.GitWebhook, error)
	UpdateGitWebhook(ctx context.Context, id string, w models.GitWebhook) error
	DeleteGitWebhook(ctx context.Context, id string) error
	RegenerateWebhookSecret(ctx context.Context, id string) (string, error)
	UpdateGitWebhookLastTriggered(ctx context.Context, id string) error
	CreateWebhookExecution(ctx context.Context, e models.GitWebhookExecution) (*models.GitWebhookExecution, error)
	UpdateWebhookExecutionCommandID(ctx context.Context, execID, commandID string) error
	UpdateWebhookExecutionStatus(ctx context.Context, id, status string, completedAt *time.Time) error
	UpdateWebhookExecutionByCommandID(ctx context.Context, commandID, status string) (webhookID string, notifyOnSuccess bool, notifyOnFailure bool, channels []string, err error)
	GetRunningExecutionForWebhook(ctx context.Context, webhookID string) (bool, error)
	ListWebhookExecutions(ctx context.Context, webhookID string, limit int) ([]models.GitWebhookExecution, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the git-webhook use-cases.
type Service struct {
	repo       Repository
	cfg        *config.Config
	dispatcher Dispatcher
	notifHub   *ws.NotificationHub
	bgCtx      context.Context
}

func NewService(repo Repository, cfg *config.Config, dispatcher Dispatcher, notifHub *ws.NotificationHub) *Service {
	return &Service{repo: repo, cfg: cfg, dispatcher: dispatcher, notifHub: notifHub, bgCtx: context.Background()}
}

// SetBackgroundContext threads a long-lived (SIGTERM-bound) ctx for the
// fire-and-forget completion callbacks.
func (s *Service) SetBackgroundContext(ctx context.Context) { s.bgCtx = ctx }

// ===== CRUD =====

// List returns all webhooks (never nil).
func (s *Service) List(ctx context.Context) ([]models.GitWebhook, error) {
	webhooks, err := s.repo.ListGitWebhooks(ctx)
	if err != nil {
		return nil, err
	}
	if webhooks == nil {
		webhooks = []models.GitWebhook{}
	}
	return webhooks, nil
}

// Create validates and stores a new webhook.
func (s *Service) Create(ctx context.Context, req models.GitWebhookRequest) (*models.GitWebhook, error) {
	if err := validateWebhookReq(req, true); err != nil {
		return nil, err
	}
	wh := req.ToModel()
	if wh.EventFilter == "" {
		wh.EventFilter = "push"
	}
	if wh.NotifyChannels == nil {
		wh.NotifyChannels = []string{}
	}
	return s.repo.CreateGitWebhook(ctx, wh)
}

// Get returns a webhook with its recent executions, or apperr.NotFound.
func (s *Service) Get(ctx context.Context, id string) (*models.GitWebhook, []models.GitWebhookExecution, error) {
	wh, err := s.repo.GetGitWebhookByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, nil, apperr.NotFound("webhook not found")
	}
	if err != nil {
		return nil, nil, err
	}
	execs, _ := s.repo.ListWebhookExecutions(ctx, id, 20)
	if execs == nil {
		execs = []models.GitWebhookExecution{}
	}
	return wh, execs, nil
}

// Update validates and applies changes to a webhook.
func (s *Service) Update(ctx context.Context, id string, req models.GitWebhookRequest) error {
	if err := validateWebhookReq(req, false); err != nil {
		return err
	}
	return s.repo.UpdateGitWebhook(ctx, id, req.ToModel())
}

// Delete removes a webhook.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteGitWebhook(ctx, id)
}

// RegenerateSecret rotates a webhook's signing secret.
func (s *Service) RegenerateSecret(ctx context.Context, id string) (string, error) {
	return s.repo.RegenerateWebhookSecret(ctx, id)
}

// Executions returns a webhook's execution history (never nil).
func (s *Service) Executions(ctx context.Context, id string, limit int) ([]models.GitWebhookExecution, error) {
	execs, err := s.repo.ListWebhookExecutions(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	if execs == nil {
		execs = []models.GitWebhookExecution{}
	}
	return execs, nil
}

func validateWebhookReq(req models.GitWebhookRequest, create bool) error {
	if req.Name == "" {
		return apperr.Validation("name is required")
	}
	if !validWebhookProviders[req.Provider] {
		if create {
			return apperr.Validation("invalid provider; must be github, gitlab, gitea, forgejo, or custom")
		}
		return apperr.Validation("invalid provider")
	}
	if create {
		if req.HostID == "" {
			return apperr.Validation("host_id is required")
		}
		if req.CustomTaskID == "" {
			return apperr.Validation("custom_task_id is required")
		}
		return nil
	}
	if req.HostID == "" || req.CustomTaskID == "" {
		return apperr.Validation("host_id and custom_task_id are required")
	}
	return nil
}

// ===== public receiver =====

// ReceiveResult is the outcome of a webhook delivery.
type ReceiveResult struct {
	Status      string // "dispatched" | "skipped"
	Reason      string // set when skipped
	ExecutionID string // set when dispatched
	CommandID   string // set when dispatched
}

// Receive verifies the signature, applies filters and dispatches a custom command
// to the agent. body is the raw request body; headers carries the provider's
// event + signature headers.
func (s *Service) Receive(ctx context.Context, id string, body []byte, headers http.Header, clientIP string) (*ReceiveResult, error) {
	wh, err := s.repo.GetGitWebhookForReceive(ctx, id)
	if err == sql.ErrNoRows || (err == nil && !wh.Enabled) {
		return nil, apperr.NotFound("webhook not found")
	}
	if err != nil {
		return nil, apperr.Internal(err)
	}

	if !verifyWebhookSignature(wh.Provider, wh.Secret, body, headers) {
		slog.ErrorContext(ctx, "webhook signature verification failed", slog.String("id", id), slog.String("ip", clientIP))
		return nil, apperr.Unauthorized("invalid signature")
	}

	eventHeader := firstNonEmpty(
		headers.Get("X-GitHub-Event"),
		headers.Get("X-Gitea-Event"),
		headers.Get("X-Forgejo-Event"),
		headers.Get("X-Gitlab-Event"),
		"push",
	)
	parsed, err := parseGitPayload(wh.Provider, body, eventHeader)
	if err != nil {
		slog.ErrorContext(ctx, "webhook payload parse failed", slog.String("id", id), slog.Any("err", err))
		return nil, apperr.Validation("failed to parse git payload")
	}
	parsed.EventType = normalizeEvent(eventHeader)

	if wh.EventFilter != "" && wh.EventFilter != "push" && parsed.EventType != wh.EventFilter {
		return &ReceiveResult{Status: "skipped", Reason: "event_filter"}, nil
	}
	if !matchFilter(wh.RepoFilter, parsed.RepoName) {
		return &ReceiveResult{Status: "skipped", Reason: "repo_filter"}, nil
	}
	if !matchFilter(wh.BranchFilter, parsed.Branch) {
		return &ReceiveResult{Status: "skipped", Reason: "branch_filter"}, nil
	}

	if running, _ := s.repo.GetRunningExecutionForWebhook(ctx, id); running {
		exec := newExecution(id, wh.Provider, parsed, "skipped")
		if created, _ := s.repo.CreateWebhookExecution(ctx, exec); created != nil {
			now := time.Now()
			_ = s.repo.UpdateWebhookExecutionStatus(ctx, created.ID, "skipped", &now)
		}
		return &ReceiveResult{Status: "skipped", Reason: "already_running"}, nil
	}

	createdExec, err := s.repo.CreateWebhookExecution(ctx, newExecution(id, wh.Provider, parsed, "pending"))
	if err != nil {
		slog.ErrorContext(ctx, "webhook execution record failed", slog.String("id", id), slog.Any("err", err))
		return nil, apperr.Internal(err)
	}

	envPayload, _ := json.Marshal(map[string]any{"env": map[string]string{
		"SS_REPO_NAME":      parsed.RepoName,
		"SS_BRANCH":         parsed.Branch,
		"SS_COMMIT_SHA":     parsed.CommitSHA,
		"SS_COMMIT_MESSAGE": parsed.CommitMessage,
		"SS_PUSHER":         parsed.Pusher,
		"SS_WEBHOOK_NAME":   wh.Name,
		"SS_EVENT_TYPE":     parsed.EventType,
	}})

	actor := fmt.Sprintf("webhook:%s", wh.Name)
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      wh.HostID,
		Module:      "custom",
		Action:      "run",
		Target:      wh.CustomTaskID,
		Payload:     string(envPayload),
		TriggeredBy: actor,
		Audit: &dispatch.AuditLogRequest{
			Username:  actor,
			Action:    "webhook_trigger",
			HostID:    wh.HostID,
			IPAddress: clientIP,
			Details:   fmt.Sprintf(`{"webhook_id":%q,"repo":%q,"branch":%q,"commit":%q}`, id, parsed.RepoName, parsed.Branch, parsed.CommitSHA),
		},
	})
	if err != nil {
		slog.ErrorContext(ctx, "webhook command dispatch failed", slog.String("id", id), slog.Any("err", err))
		now := time.Now()
		_ = s.repo.UpdateWebhookExecutionStatus(ctx, createdExec.ID, "failed", &now)
		return nil, apperr.Internal(err)
	}

	_ = s.repo.UpdateWebhookExecutionCommandID(ctx, createdExec.ID, result.Command.ID)
	_ = s.repo.UpdateGitWebhookLastTriggered(ctx, id)
	slog.InfoContext(ctx, "webhook dispatched",
		slog.String("id", id), slog.String("command", result.Command.ID),
		slog.String("host", wh.HostID), slog.String("repo", parsed.RepoName), slog.String("branch", parsed.Branch))

	return &ReceiveResult{Status: "dispatched", ExecutionID: createdExec.ID, CommandID: result.Command.ID}, nil
}

func newExecution(id, provider string, p *parsedGitPayload, status string) models.GitWebhookExecution {
	return models.GitWebhookExecution{
		WebhookID:     id,
		Provider:      provider,
		RepoName:      p.RepoName,
		Branch:        p.Branch,
		CommitSHA:     p.CommitSHA,
		CommitMessage: p.CommitMessage,
		Pusher:        p.Pusher,
		Status:        status,
	}
}

// ===== completion notification =====

// NotifyComplete updates a webhook execution's status and fans out notifications
// when its triggered command completes/fails. Safe to call in a goroutine.
func (s *Service) NotifyComplete(commandID, status string) {
	ctx := s.bgCtx
	webhookID, notifyOnSuccess, notifyOnFailure, channels, err := s.repo.UpdateWebhookExecutionByCommandID(ctx, commandID, status)
	if err != nil {
		return // not a webhook-triggered command
	}
	shouldNotify := (status == "completed" && notifyOnSuccess) || (status == "failed" && notifyOnFailure)
	if !shouldNotify || len(channels) == 0 {
		return
	}
	wh, err := s.repo.GetGitWebhookForReceive(ctx, webhookID)
	if err != nil {
		return
	}

	emoji := "✅"
	if status == "failed" {
		emoji = "❌"
	}
	subject := fmt.Sprintf("[ServerSupervisor] Webhook %s %s %s", wh.Name, emoji, status)
	msg := fmt.Sprintf("Webhook '%s' execution %s on host %s (task: %s)", wh.Name, status, wh.HostID, wh.CustomTaskID)

	n := notify.New()
	for _, ch := range channels {
		switch ch {
		case "smtp":
			if s.cfg.SMTPTo == "" || s.cfg.SMTPFrom == "" {
				continue
			}
			if err := n.SendSMTP(s.cfg, s.cfg.SMTPFrom, s.cfg.SMTPTo, subject, msg); err != nil {
				slog.ErrorContext(ctx, "webhook SMTP send", slog.Any("err", err))
			}
		case "ntfy":
			if s.cfg.NotifyURL == "" {
				continue
			}
			if err := n.SendNtfy(s.cfg, s.cfg.NotifyURL, subject, msg); err != nil {
				slog.ErrorContext(ctx, "webhook ntfy send", slog.Any("err", err))
			}
		case "browser":
			if s.notifHub == nil {
				continue
			}
			s.notifHub.Broadcast(models.WSWebhookExecutionMessage{
				Type: "webhook_execution",
				Notification: models.WSWebhookNotification{
					WebhookID:   webhookID,
					WebhookName: wh.Name,
					Status:      status,
					TriggeredAt: time.Now().UTC(),
				},
			})
		}
	}
}

// ===== signature / parsing helpers =====

func verifyWebhookSignature(provider, secret string, body []byte, headers http.Header) bool {
	switch provider {
	case "gitlab":
		return hmac.Equal([]byte(headers.Get("X-Gitlab-Token")), []byte(secret))
	default:
		sigHeader := firstNonEmpty(
			headers.Get("X-Hub-Signature-256"),
			headers.Get("X-Gitea-Signature"),
			headers.Get("X-Forgejo-Signature"),
		)
		const prefix = "sha256="
		if !strings.HasPrefix(sigHeader, prefix) {
			return false
		}
		gotBytes, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
		if err != nil {
			return false
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		return hmac.Equal(gotBytes, mac.Sum(nil))
	}
}

type parsedGitPayload struct {
	RepoName      string
	Branch        string
	CommitSHA     string
	CommitMessage string
	Pusher        string
	EventType     string
}

func parseGitPayload(provider string, body []byte, eventHeader string) (*parsedGitPayload, error) {
	if provider == "gitlab" {
		return parseGitLabPayload(body)
	}
	return parseGitHubPayload(body, eventHeader)
}

func parseGitHubPayload(body []byte, _ string) (*parsedGitPayload, error) {
	var p struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
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
	result := &parsedGitPayload{Branch: refToBranch(p.Ref), CommitSHA: p.After}
	if p.Repository != nil {
		result.RepoName = p.Repository.FullName
	}
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
	result := &parsedGitPayload{Branch: refToBranch(p.Ref), CommitSHA: p.After, Pusher: p.UserName}
	if p.Project != nil {
		result.RepoName = p.Project.PathWithNamespace
	}
	if len(p.Commits) > 0 {
		last := p.Commits[len(p.Commits)-1]
		result.CommitMessage = firstLine(last.Message)
		if result.CommitSHA == "" {
			result.CommitSHA = last.ID
		}
	}
	return result, nil
}

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
	if filter == "" || filter == value {
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
