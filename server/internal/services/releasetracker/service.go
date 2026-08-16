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
	// versionhelpers is internal/releasetracker, the pure version-comparison
	// helpers; aliased because this package shares its name.
	versionhelpers "github.com/serversupervisor/server/internal/releasetracker"
	"github.com/serversupervisor/server/internal/safego"
	"github.com/serversupervisor/server/internal/services/dockerversions"
	"github.com/serversupervisor/server/internal/services/notifychannels"
	"github.com/serversupervisor/server/internal/services/push"
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
	ListPickableContainers(ctx context.Context) ([]models.TrackableContainer, error)
	ListTrackerTagDigests(ctx context.Context, trackerID string, limit int) ([]models.ReleaseVersionHistoryItem, error)
	UpdateReleaseTrackerExecutionByCommandID(ctx context.Context, commandID, status string) (trackerID string, notifyOnRelease bool, channels []string, err error)
	TrackerDriftDetected(ctx context.Context, t models.ReleaseTracker) (bool, error)
}

// Service holds the release-tracker HTTP use-cases + owns the background poller.
type Service struct {
	repo     Repository
	cfg      *config.Config
	notifHub *ws.NotificationHub
	dispatch *notifychannels.Dispatcher
	poller   *Poller
	images   *dockerversions.Service
}

func NewService(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, notifHub *ws.NotificationHub, pushSvc *push.Service) *Service {
	// One shared ambient engine instance: the release-tracker poller reads it
	// on every docker tracker check, and main.go ticks its own background sweep
	// through RefreshImageVersions below — same object, so the two never issue
	// duplicate registry calls for the same image (see dockerversions.Service).
	images := dockerversions.NewService(db, cfg)
	return &Service{
		repo:     db,
		cfg:      cfg,
		notifHub: notifHub,
		dispatch: notifychannels.NewDispatcher(cfg, pushSvc),
		poller:   NewPoller(db, cfg, dispatcher, notifHub, pushSvc, images),
		images:   images,
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

// ImagePollInterval is the ambient image-version engine's cadence (default 6h).
func (s *Service) ImagePollInterval() time.Duration {
	if s.images == nil {
		return 6 * time.Hour
	}
	return s.images.RefreshInterval()
}

// RefreshImageVersions runs one ambient sweep of every running image (poller.Every
// ticks this from main, same handler-exposes-a-poll-once shape as CheckAll).
func (s *Service) RefreshImageVersions(ctx context.Context) {
	if s.images == nil {
		return
	}
	s.images.RefreshAll(ctx)
}

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
	for i := range trackers {
		trackers[i].DriftDetected, _ = s.repo.TrackerDriftDetected(ctx, trackers[i])
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
	t.DriftDetected, _ = s.repo.TrackerDriftDetected(ctx, *t)
	return t, execs, nil
}

// TrackableContainers backs the bulk "activer la mise à jour auto" flow:
// compose-managed containers without a tracker yet, the only ones a compose
// pull+up can actually auto-update.
func (s *Service) TrackableContainers(ctx context.Context) ([]models.TrackableContainer, error) {
	return s.repo.ListTrackableContainers(ctx)
}

// PickableContainers backs the single-tracker container picker that replaced
// the free-text docker_image/docker_tag inputs — every running container, with
// no compose restriction (a manual tracker only needs an image reference).
func (s *Service) PickableContainers(ctx context.Context) ([]models.TrackableContainer, error) {
	return s.repo.ListPickableContainers(ctx)
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
// provider history). A docker tracker with an optional git link keeps the digest
// history as its spine — that's the deployed truth — and only borrows the
// matching release's title/URL/date from the provider, so linking a repo never
// changes which versions are listed, just how much context each one carries.
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
		s.enrichWithGitReleases(ctx, *t, history, limit)
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

// enrichWithGitReleases fills in release notes for a docker tracker that opted
// into a git link. Best-effort by design: an unreachable or wrong repo must not
// fail the digest history that already works without it, so failures are
// swallowed and the untouched history is returned.
func (s *Service) enrichWithGitReleases(ctx context.Context, t models.ReleaseTracker, history []models.ReleaseVersionHistoryItem, limit int) {
	if !hasGitLink(t) || len(history) == 0 {
		return
	}
	releases, err := gitprovider.NewClient(t.Provider, s.cfg.GitHubToken).FetchReleaseHistory(t.RepoOwner, t.RepoName, limit)
	if err != nil {
		slog.WarnContext(ctx, fmt.Sprintf("Docker tracker %s: linked repo %s/%s release notes unavailable: %v", t.Name, t.RepoOwner, t.RepoName, err))
		return
	}
	byVersion := make(map[string]gitprovider.Release, len(releases))
	for _, r := range releases {
		byVersion[versionhelpers.NormalizeVersion(r.TagName)] = r
	}
	for i := range history {
		r, ok := byVersion[versionhelpers.NormalizeVersion(history[i].Version)]
		if !ok {
			continue
		}
		history[i].Name = r.Name
		history[i].ReleaseURL = r.HTMLURL
		if !r.PublishedAt.IsZero() {
			published := r.PublishedAt
			history[i].PublishedAt = &published
		}
	}
}

// hasGitLink reports whether a tracker names a git repository — required for a
// git tracker, optional (release notes only) for a docker one.
func hasGitLink(t models.ReleaseTracker) bool {
	return t.RepoOwner != "" && t.RepoName != "" && validReleaseProviders[t.Provider]
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
	go func() {
		defer safego.Recover(pollCtx, "releasetracker.TriggerCheck")
		s.poller.CheckOne(pollCtx, *t)
	}()
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
		go func() {
			defer safego.Recover(pollCtx, "releasetracker.Run.dispatchDocker")
			s.poller.DispatchDockerTracker(pollCtx, *t, tag, t.LastReleaseTag, t.LatestImageDigest, t.LatestImageDigest)
		}()
		return nil
	}
	if t.HostID == "" || t.CustomTaskID == "" {
		return apperr.Conflict("tracker en mode surveillance seule — configurez une VM cible et une tâche pour déclencher manuellement")
	}
	if t.LastReleaseTag == "" {
		return apperr.Conflict("aucune release initiale enregistrée — attendez le prochain cycle de polling avant de déclencher manuellement")
	}
	go func() {
		defer safego.Recover(pollCtx, "releasetracker.Run.dispatchGit")
		s.poller.DispatchGitRelease(pollCtx, *t, t.LastReleaseTag, "", "")
	}()
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

	statusLabel := "réussie"
	trackerLabel := "Git"
	if tracker.TrackerType == "docker" {
		trackerLabel = "Docker"
	}
	if status == "failed" {
		statusLabel = "échouée"
	}

	s.dispatch.Send(ctx, notifychannels.Event{
		LogID:       "tracker:" + tracker.ID,
		Channels:    channels,
		SMTPSubject: subject,
		SMTPBody:    msg,
		SMTPTo:      s.cfg.SMTPTo,
		NtfyTitle:   subject,
		NtfyBody:    msg,
		NtfyURL:     s.cfg.NotifyURL,
		OnBrowser: func() {
			if s.notifHub == nil {
				return
			}
			s.notifHub.Broadcast(models.WSReleaseTrackerMessage{
				Type: "release_tracker_execution",
				Notification: models.WSReleaseTrackerNotification{
					TrackerID: tracker.ID, TrackerName: tracker.Name, TrackerType: tracker.TrackerType,
					Status: status, TriggeredAt: time.Now().UTC(),
				},
			})
		},
		Push: &push.Payload{
			Title:  fmt.Sprintf("%s tracker : %s", trackerLabel, tracker.Name),
			Body:   fmt.Sprintf("Exécution %s", statusLabel),
			Tag:    fmt.Sprintf("tracker-exec-%s-%s", tracker.ID, status),
			URL:    fmt.Sprintf("/release-trackers/%s", tracker.ID),
			Status: status,
		},
	})
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
	// Still required, and still stored — the update action needs to know exactly
	// which image:tag it targets. It is no longer typed by hand though: the UI
	// fills it from a picked running container (ListPickableContainers).
	if req.DockerImage == "" {
		return "docker_image is required for docker trackers (select a running container)"
	}
	if req.DockerTag == "" {
		req.DockerTag = "latest"
	}
	// Optional git link, release notes only (see Service.enrichWithGitReleases).
	if msg := validateOptionalGitLink(req); msg != "" {
		return msg
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

// validateOptionalGitLink checks the optional repo a docker tracker may name to
// surface release notes alongside its digest history. Leaving all three fields
// empty is valid (no link); naming one without the others is not. Provider is
// defaulted to github rather than rejected, matching the git-tracker create path.
func validateOptionalGitLink(req *models.ReleaseTracker) string {
	if req.RepoOwner == "" && req.RepoName == "" {
		req.Provider = "" // don't persist a dangling provider with no repo
		return ""
	}
	if req.RepoOwner == "" || req.RepoName == "" {
		return "repo_owner and repo_name must be provided together for the optional git link"
	}
	if req.Provider == "" {
		req.Provider = "github"
	}
	if !validReleaseProviders[req.Provider] {
		return "invalid provider; must be github, gitlab, or gitea"
	}
	return ""
}
