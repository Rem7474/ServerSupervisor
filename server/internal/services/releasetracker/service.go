package releasetracker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/ws"
)

var validReleaseProviders = map[string]bool{"github": true, "gitlab": true, "gitea": true}

// Repository is the data-access port for the HTTP use-cases + completion notify.
// (The poller writes through the concrete *database.DB directly.)
type Repository interface {
	ListRegistryCredentials(ctx context.Context) ([]models.RegistryCredential, error)
	CreateRegistryCredential(ctx context.Context, rc models.RegistryCredential) (*models.RegistryCredential, error)
	UpdateRegistryCredential(ctx context.Context, id string, rc models.RegistryCredential) error
	DeleteRegistryCredential(ctx context.Context, id string) error
	ListReleaseTrackers(ctx context.Context) ([]models.ReleaseTracker, error)
	CreateReleaseTracker(ctx context.Context, t models.ReleaseTracker) (*models.ReleaseTracker, error)
	GetReleaseTrackerByID(ctx context.Context, id string) (*models.ReleaseTracker, error)
	UpdateReleaseTracker(ctx context.Context, id string, t models.ReleaseTracker) error
	DeleteReleaseTracker(ctx context.Context, id string) error
	ListReleaseTrackerExecutions(ctx context.Context, trackerID string, limit int) ([]models.ReleaseTrackerExecution, error)
	ListTrackableContainers(ctx context.Context) ([]models.TrackableContainer, error)
	ListTrackerTagDigests(ctx context.Context, trackerID string, limit int) ([]models.ReleaseVersionHistoryItem, error)
	UpdateReleaseTrackerExecutionByCommandID(ctx context.Context, commandID, status string) (trackerID string, notifyOnRelease bool, channels []string, err error)
}

// Service holds the release-tracker HTTP use-cases + owns the background poller.
type Service struct {
	repo     Repository
	cfg      *config.Config
	notifHub *ws.NotificationHub
	poller   *Poller
}

func NewService(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub) *Service {
	return &Service{
		repo:     db,
		cfg:      cfg,
		notifHub: notifHub,
		poller:   NewPoller(db, cfg, dispatcher, notifHub),
	}
}

// PollInterval returns the configured poll cadence (default 15m).
func (s *Service) PollInterval() time.Duration {
	if s.cfg.GitHubPollInterval == 0 {
		return 15 * time.Minute
	}
	return s.cfg.GitHubPollInterval
}

// CheckAll polls every enabled tracker once (poller.Every ticks this).
func (s *Service) CheckAll(ctx context.Context) { s.poller.CheckAll(ctx) }

// ===== registry credentials =====

func (s *Service) ListRegistryCredentials(ctx context.Context) ([]models.RegistryCredential, error) {
	return s.repo.ListRegistryCredentials(ctx)
}

func (s *Service) CreateRegistryCredential(ctx context.Context, req models.RegistryCredentialRequest) (*models.RegistryCredential, error) {
	if req.Name == "" || req.RegistryHost == "" || req.Username == "" || req.Password == "" {
		return nil, apperr.Validation("name, registry_host, username and password are required")
	}
	created, err := s.repo.CreateRegistryCredential(ctx, req.ToModel())
	if err != nil {
		return nil, err
	}
	created.Password = "" // never echo the secret
	return created, nil
}

func (s *Service) UpdateRegistryCredential(ctx context.Context, id string, req models.RegistryCredentialRequest) error {
	if req.Name == "" || req.RegistryHost == "" || req.Username == "" {
		return apperr.Validation("name, registry_host and username are required")
	}
	return s.repo.UpdateRegistryCredential(ctx, id, req.ToModel())
}

func (s *Service) DeleteRegistryCredential(ctx context.Context, id string) error {
	return s.repo.DeleteRegistryCredential(ctx, id)
}

// ===== trackers CRUD =====

func (s *Service) List(ctx context.Context) ([]models.ReleaseTracker, error) {
	trackers, err := s.repo.ListReleaseTrackers(ctx)
	if err != nil {
		return nil, err
	}
	if trackers == nil {
		trackers = []models.ReleaseTracker{}
	}
	return trackers, nil
}

func (s *Service) Create(ctx context.Context, req models.ReleaseTrackerRequest) (*models.ReleaseTracker, error) {
	m := req.ToModel()
	if msg := validateTracker(&m, true); msg != "" {
		return nil, apperr.Validation(msg)
	}
	return s.repo.CreateReleaseTracker(ctx, m)
}

func (s *Service) Update(ctx context.Context, id string, req models.ReleaseTrackerRequest) error {
	m := req.ToModel()
	if msg := validateTracker(&m, false); msg != "" {
		return apperr.Validation(msg)
	}
	return s.repo.UpdateReleaseTracker(ctx, id, m)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteReleaseTracker(ctx, id)
}

// BulkResult reports one entry's outcome in a bulk create.
type BulkResult struct {
	Name    string `json:"name"`
	Created bool   `json:"created"`
	Error   string `json:"error,omitempty"`
}

// CreateBulk creates many docker trackers; each entry is validated independently.
func (s *Service) CreateBulk(ctx context.Context, reqs []models.ReleaseTrackerRequest) (int, []BulkResult, error) {
	if len(reqs) == 0 {
		return 0, nil, apperr.Validation("trackers array is required")
	}
	if len(reqs) > 100 {
		return 0, nil, apperr.Validation("too many trackers (max 100)")
	}
	results := make([]BulkResult, 0, len(reqs))
	created := 0
	for _, reqT := range reqs {
		m := reqT.ToModel()
		m.TrackerType = "docker"
		if m.Name == "" {
			results = append(results, BulkResult{Name: m.Name, Error: "name is required"})
			continue
		}
		if m.CooldownHours < 0 || m.CooldownHours > 168 {
			results = append(results, BulkResult{Name: m.Name, Error: "cooldown_hours must be between 0 and 168"})
			continue
		}
		if msg := validateDockerTracker(&m); msg != "" {
			results = append(results, BulkResult{Name: m.Name, Error: msg})
			continue
		}
		if m.NotifyChannels == nil {
			m.NotifyChannels = []string{}
		}
		if _, err := s.repo.CreateReleaseTracker(ctx, m); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("CreateBulk: failed to create %q: %v", m.Name, err))
			results = append(results, BulkResult{Name: m.Name, Error: "failed to create"})
			continue
		}
		created++
		results = append(results, BulkResult{Name: m.Name, Created: true})
	}
	return created, results, nil
}

func (s *Service) Get(ctx context.Context, id string) (*models.ReleaseTracker, []models.ReleaseTrackerExecution, error) {
	t, err := s.repo.GetReleaseTrackerByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, nil, apperr.NotFound("tracker not found")
	}
	if err != nil {
		return nil, nil, err
	}
	execs, _ := s.repo.ListReleaseTrackerExecutions(ctx, id, 20)
	if execs == nil {
		execs = []models.ReleaseTrackerExecution{}
	}
	return t, execs, nil
}

func (s *Service) TrackableContainers(ctx context.Context) ([]models.TrackableContainer, error) {
	return s.repo.ListTrackableContainers(ctx)
}

func (s *Service) Executions(ctx context.Context, id string) ([]models.ReleaseTrackerExecution, error) {
	execs, err := s.repo.ListReleaseTrackerExecutions(ctx, id, 50)
	if err != nil {
		return nil, err
	}
	if execs == nil {
		execs = []models.ReleaseTrackerExecution{}
	}
	return execs, nil
}

// VersionHistory returns the recent versions (docker: stored digests; git: live
// provider history).
func (s *Service) VersionHistory(ctx context.Context, id string, limit int) ([]models.ReleaseVersionHistoryItem, error) {
	t, err := s.repo.GetReleaseTrackerByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound("tracker not found")
	}
	if err != nil {
		return nil, err
	}
	history := make([]models.ReleaseVersionHistoryItem, 0)
	if t.TrackerType == "docker" {
		history, err = s.repo.ListTrackerTagDigests(ctx, id, limit)
		if err != nil {
			return nil, err
		}
	} else {
		releases, ferr := gitprovider.NewClient(t.Provider, s.cfg.GitHubToken).FetchReleaseHistory(t.RepoOwner, t.RepoName, limit)
		if ferr != nil {
			return nil, apperr.BadGateway(ferr.Error())
		}
		for _, r := range releases {
			item := models.ReleaseVersionHistoryItem{Version: r.TagName, Name: r.Name, ReleaseURL: r.HTMLURL}
			if !r.PublishedAt.IsZero() {
				published := r.PublishedAt
				item.PublishedAt = &published
			}
			history = append(history, item)
		}
	}
	if history == nil {
		history = []models.ReleaseVersionHistoryItem{}
	}
	return history, nil
}

// ===== check-now / manual run (delegated to the poller on pollCtx) =====

// TriggerCheck schedules an immediate poll of one tracker.
func (s *Service) TriggerCheck(reqCtx, pollCtx context.Context, id string) error {
	t, err := s.repo.GetReleaseTrackerByID(reqCtx, id)
	if err == sql.ErrNoRows {
		return apperr.NotFound("tracker not found")
	}
	if err != nil {
		return err
	}
	go s.poller.CheckOne(pollCtx, *t)
	return nil
}

// Run manually triggers the tracker's task with the last known release info.
func (s *Service) Run(reqCtx, pollCtx context.Context, id string) error {
	t, err := s.repo.GetReleaseTrackerByID(reqCtx, id)
	if err == sql.ErrNoRows {
		return apperr.NotFound("tracker not found")
	}
	if err != nil {
		return err
	}
	if t.TrackerType == "docker" {
		if t.UpdateAction == "compose" && !trackerHasDispatchTarget(*t) {
			return apperr.Conflict("mode compose : configurez une VM cible et un projet compose pour déclencher manuellement")
		}
		if t.LatestImageDigest == "" {
			return apperr.Conflict("aucune vérification initiale effectuée — attendez le prochain cycle de polling avant de déclencher manuellement")
		}
		tag := t.DockerTag
		if tag == "" {
			tag = "latest"
		}
		go s.poller.DispatchDockerTracker(pollCtx, *t, tag, t.LastReleaseTag, t.LatestImageDigest, t.LatestImageDigest)
		return nil
	}
	if t.HostID == "" || t.CustomTaskID == "" {
		return apperr.Conflict("tracker en mode surveillance seule — configurez une VM cible et une tâche pour déclencher manuellement")
	}
	if t.LastReleaseTag == "" {
		return apperr.Conflict("aucune release initiale enregistrée — attendez le prochain cycle de polling avant de déclencher manuellement")
	}
	go s.poller.DispatchGitRelease(pollCtx, *t, t.LastReleaseTag, "", "")
	return nil
}

// ===== completion notification =====

// NotifyComplete updates a tracker execution + fans out notifications when its
// command completes. Called fire-and-forget on the detached ctx.
func (s *Service) NotifyComplete(ctx context.Context, commandID, status string) {
	trackerID, notifyOnRelease, channels, err := s.repo.UpdateReleaseTrackerExecutionByCommandID(ctx, commandID, status)
	if err != nil {
		return // not a tracker command
	}
	if !notifyOnRelease || len(channels) == 0 {
		return
	}
	tracker, err := s.repo.GetReleaseTrackerByID(ctx, trackerID)
	if err != nil {
		return
	}

	emoji := "✅"
	if status == "failed" {
		emoji = "❌"
	}
	var subject, msg string
	if tracker.TrackerType == "docker" {
		imageFull := tracker.DockerImage + ":" + tracker.DockerTag
		if tracker.DockerTag == "" {
			imageFull = tracker.DockerImage + ":latest"
		}
		subject = fmt.Sprintf("[ServerSupervisor] Docker tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Docker tracker '%s' (%s) execution %s on host %s (task: %s)", tracker.Name, imageFull, status, tracker.HostID, tracker.CustomTaskID)
	} else {
		subject = fmt.Sprintf("[ServerSupervisor] Release tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Release tracker '%s' (%s/%s) execution %s on host %s (task: %s)", tracker.Name, tracker.RepoOwner, tracker.RepoName, status, tracker.HostID, tracker.CustomTaskID)
	}

	notifier := notify.New()
	for _, ch := range channels {
		switch ch {
		case "smtp":
			if s.cfg.SMTPTo == "" || s.cfg.SMTPFrom == "" {
				continue
			}
			if err := notifier.SendSMTP(s.cfg, s.cfg.SMTPFrom, s.cfg.SMTPTo, subject, msg); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Release tracker SMTP send: %v", err))
			}
		case "ntfy":
			if s.cfg.NotifyURL == "" {
				continue
			}
			if err := notifier.SendNtfy(s.cfg, s.cfg.NotifyURL, subject, msg); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Release tracker notify ntfy: %v", err))
			}
		case "browser":
			if s.notifHub == nil {
				continue
			}
			s.notifHub.Broadcast(models.WSReleaseTrackerMessage{
				Type: "release_tracker_execution",
				Notification: models.WSReleaseTrackerNotification{
					TrackerID: tracker.ID, TrackerName: tracker.Name, TrackerType: tracker.TrackerType,
					Status: status, TriggeredAt: time.Now().UTC(),
				},
			})
		}
	}
}

// ===== validation =====

// validateTracker normalizes defaults and validates a tracker. Returns "" when
// valid, otherwise a 400 message. isCreate toggles the create-only provider
// defaulting + message.
func validateTracker(m *models.ReleaseTracker, isCreate bool) string {
	if m.TrackerType == "" {
		m.TrackerType = "git"
	}
	if m.TrackerType != "git" && m.TrackerType != "docker" {
		return "tracker_type must be 'git' or 'docker'"
	}
	if m.Name == "" {
		return "name is required"
	}
	if m.CooldownHours < 0 || m.CooldownHours > 168 {
		return "cooldown_hours must be between 0 and 168"
	}
	if m.TrackerType == "git" {
		if (m.HostID == "") != (m.CustomTaskID == "") {
			return "host_id and custom_task_id must be provided together for git trackers"
		}
		if m.RepoOwner == "" || m.RepoName == "" {
			return "repo_owner and repo_name are required for git trackers"
		}
		if isCreate && m.Provider == "" {
			m.Provider = "github"
		}
		if !validReleaseProviders[m.Provider] {
			if isCreate {
				return "invalid provider; must be github, gitlab, or gitea"
			}
			return "invalid provider"
		}
	} else {
		if msg := validateDockerTracker(m); msg != "" {
			return msg
		}
	}
	if m.NotifyChannels == nil {
		m.NotifyChannels = []string{}
	}
	return ""
}

// validateDockerTracker normalizes + validates a docker tracker's deployment mode.
// Returns "" when valid.
func validateDockerTracker(req *models.ReleaseTracker) string {
	if req.UpdateAction == "" {
		req.UpdateAction = "custom"
	}
	if req.UpdateAction != "custom" && req.UpdateAction != "compose" {
		return "update_action must be 'custom' or 'compose'"
	}
	if req.DockerImage == "" {
		return "docker_image is required for docker trackers"
	}
	if req.DockerTag == "" {
		req.DockerTag = "latest"
	}
	if req.UpdateAction == "compose" {
		if req.HostID == "" || req.ComposeProject == "" {
			return "host_id and compose_project are required for compose update mode"
		}
	} else if req.HostID != "" && req.CustomTaskID == "" {
		return "custom_task_id is required when host_id is set"
	}
	if req.HealthcheckTimeoutSec < 0 || req.HealthcheckTimeoutSec > 3600 {
		return "healthcheck_timeout_sec must be between 0 and 3600"
	}
	return ""
}
