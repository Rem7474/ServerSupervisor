// Package dockerversions is the ambient Docker image-version engine: the single
// place in the server that asks an OCI registry "what is the latest digest for
// image X, tag Y".
//
// It exists because that question used to be answered once per manually created
// release_trackers row, inside the release-tracker poller — so a container with
// no tracker never got a version badge, and two trackers on the same image
// polled the same registry twice. The engine refreshes a cache table
// (docker_image_versions) on a slow ticker for every distinct image:tag actually
// running across the fleet, and both consumers now read that table instead:
//
//   - ws.buildVersionComparisons → a version badge for EVERY container.
//   - releasetracker.Poller.checkOneDocker → detection/dispatch only, no HTTP.
//
// Registry access itself is unchanged and still lives in internal/gitprovider
// (GHCR / Docker Hub / any WWW-Authenticate-discoverable v2 registry); this
// package owns *when* those calls happen and *what* gets cached.
package dockerversions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/gitprovider"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListDistinctDockerImages(ctx context.Context) ([]models.DockerImageRef, error)
	ListDockerTrackerImageRefs(ctx context.Context) ([]models.DockerImageRef, error)
	ListRegistryCredentials(ctx context.Context) ([]models.RegistryCredential, error)
	GetRegistryCredentialAuth(ctx context.Context, id string) (username, password string, err error)
	GetDockerImageVersion(ctx context.Context, image, tag string) (*models.DockerImageVersion, error)
	UpsertDockerImageVersion(ctx context.Context, v models.DockerImageVersion) error
	SetDockerImageVersionError(ctx context.Context, image, tag, registry, errMsg string) error
	PruneDockerImageVersions(ctx context.Context, keep []models.DockerImageRef) (int64, error)
}

// RegistryClient isolates the two gitprovider registry calls the engine makes,
// so the refresh logic is unit-testable without any network.
type RegistryClient interface {
	ManifestDigest(image, tag, token, username, password string) (string, error)
	VersionForDigest(image, digest, token, username, password string) string
}

type gitproviderRegistry struct{}

func (gitproviderRegistry) ManifestDigest(image, tag, token, username, password string) (string, error) {
	return gitprovider.FetchDockerManifestDigestWithAuth(image, tag, token, username, password)
}

func (gitproviderRegistry) VersionForDigest(image, digest, token, username, password string) string {
	return gitprovider.FetchDockerVersionForDigestWithAuth(image, digest, token, username, password)
}

// Service refreshes and serves the docker_image_versions cache.
type Service struct {
	repo Repository
	cfg  *config.Config
	reg  RegistryClient

	// inflight serialises concurrent refreshes of the same image ref (the slow
	// background sweep vs. a tracker's on-demand read), so the engine never
	// issues two identical registry calls at once.
	mu       sync.Mutex
	inflight map[string]*sync.Mutex
}

func NewService(repo Repository, cfg *config.Config) *Service {
	return &Service{repo: repo, cfg: cfg, reg: gitproviderRegistry{}, inflight: map[string]*sync.Mutex{}}
}

// RefreshInterval is the cadence of the background sweep (default 6h).
func (s *Service) RefreshInterval() time.Duration {
	if s.cfg == nil || s.cfg.DockerImagePollInterval <= 0 {
		return 6 * time.Hour
	}
	return s.cfg.DockerImagePollInterval
}

// NormalizeTag applies the Docker default so "" and "latest" share one row.
func NormalizeTag(tag string) string {
	if strings.TrimSpace(tag) == "" {
		return "latest"
	}
	return tag
}

func refKey(image, tag string) string { return image + ":" + NormalizeTag(tag) }

// RefreshAll is the background sweep (poller.Every ticks it): one registry check
// per distinct image:tag running anywhere, plus the refs named by enabled docker
// trackers so a tracker whose container is currently stopped keeps working.
// Sequential on purpose — registries rate-limit per source IP, and this runs on
// a multi-hour cadence where wall-clock duration doesn't matter.
func (s *Service) RefreshAll(ctx context.Context) {
	refs, err := s.workList(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "docker image versions: failed to build work list", slog.Any("err", err))
		return
	}
	creds, err := s.repo.ListRegistryCredentials(ctx)
	if err != nil {
		slog.WarnContext(ctx, "docker image versions: failed to load registry credentials", slog.Any("err", err))
		creds = nil
	}

	checked := 0
	for _, ref := range refs {
		if ctx.Err() != nil {
			return
		}
		if _, err := s.refresh(ctx, ref, creds); err != nil {
			// Already recorded on the row; a single unreachable image must not
			// abort the sweep for every other image.
			slog.DebugContext(ctx, "docker image versions: check failed",
				slog.String("image", ref.Image), slog.String("tag", ref.ImageTag), slog.Any("err", err))
		}
		checked++
	}

	if removed, err := s.repo.PruneDockerImageVersions(ctx, refs); err == nil && removed > 0 {
		slog.InfoContext(ctx, "docker image versions: pruned stale cache rows", slog.Int64("removed", removed))
	}
	slog.InfoContext(ctx, "docker image versions refreshed", slog.Int("images", checked))
}

// workList is the deduplicated union of running container images and enabled
// docker trackers' image refs.
func (s *Service) workList(ctx context.Context) ([]models.DockerImageRef, error) {
	running, err := s.repo.ListDistinctDockerImages(ctx)
	if err != nil {
		return nil, err
	}
	tracked, err := s.repo.ListDockerTrackerImageRefs(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(running)+len(tracked))
	out := make([]models.DockerImageRef, 0, len(running)+len(tracked))
	for _, ref := range append(running, tracked...) {
		if strings.TrimSpace(ref.Image) == "" {
			continue
		}
		ref.ImageTag = NormalizeTag(ref.ImageTag)
		if _, dup := seen[refKey(ref.Image, ref.ImageTag)]; dup {
			continue
		}
		seen[refKey(ref.Image, ref.ImageTag)] = struct{}{}
		out = append(out, ref)
	}
	return out, nil
}

// Latest returns the cached version info for image:tag, refreshing it on demand
// when the cache has no row yet or the row is older than one refresh interval.
// This is what makes the tracker poller's "vérifier maintenant" immediate
// without giving it a second registry code path of its own.
func (s *Service) Latest(ctx context.Context, image, tag string) (models.DockerImageVersion, error) {
	tag = NormalizeTag(tag)
	cached, err := s.repo.GetDockerImageVersion(ctx, image, tag)
	if err != nil {
		return models.DockerImageVersion{}, err
	}
	if cached != nil && !s.isStale(*cached) {
		return *cached, nil
	}

	creds, credErr := s.repo.ListRegistryCredentials(ctx)
	if credErr != nil {
		creds = nil
	}
	refreshed, err := s.refresh(ctx, models.DockerImageRef{Image: image, ImageTag: tag}, creds)
	if err != nil {
		// Fall back to the stale-but-known row rather than reporting nothing:
		// a transient registry outage shouldn't erase a digest we already have.
		if cached != nil {
			cached.LastError = err.Error()
			return *cached, nil
		}
		return models.DockerImageVersion{Image: image, ImageTag: tag, LastError: err.Error()}, err
	}
	return refreshed, nil
}

func (s *Service) isStale(v models.DockerImageVersion) bool {
	if v.CheckedAt == nil {
		return true
	}
	return time.Since(*v.CheckedAt) >= s.RefreshInterval()
}

// refresh performs one registry check and writes the result to the cache.
func (s *Service) refresh(ctx context.Context, ref models.DockerImageRef, creds []models.RegistryCredential) (models.DockerImageVersion, error) {
	ref.ImageTag = NormalizeTag(ref.ImageTag)
	unlock := s.lockRef(ref)
	defer unlock()

	registry, _ := gitprovider.ParseDockerImageRef(ref.Image)
	credID, user, pass := s.credentialsFor(ctx, registry, creds)

	digest, err := s.reg.ManifestDigest(ref.Image, ref.ImageTag, s.cfgToken(), user, pass)
	if err != nil {
		msg := friendlyRegistryError(err, ref.ImageTag, credID != "")
		_ = s.repo.SetDockerImageVersionError(ctx, ref.Image, ref.ImageTag, registry, msg)
		return models.DockerImageVersion{}, fmt.Errorf("%s", msg)
	}
	if digest == "" {
		const msg = "digest vide retourné par le registre"
		_ = s.repo.SetDockerImageVersionError(ctx, ref.Image, ref.ImageTag, registry, msg)
		return models.DockerImageVersion{}, fmt.Errorf("%s", msg)
	}

	// Resolve a mutable/broad tag ("latest", "v5") to the exact release behind
	// it when the registry exposes one; a pinned tag is already its own version.
	resolved := ref.ImageTag
	if ShouldResolveTag(ref.ImageTag) {
		if v := s.reg.VersionForDigest(ref.Image, digest, s.cfgToken(), user, pass); v != "" {
			resolved = v
		}
	}

	out := models.DockerImageVersion{
		Image: ref.Image, ImageTag: ref.ImageTag, Registry: registry,
		LatestDigest: digest, LatestTag: resolved, RegistryCredentialsID: credID,
	}
	if err := s.repo.UpsertDockerImageVersion(ctx, out); err != nil {
		return models.DockerImageVersion{}, err
	}
	now := time.Now()
	out.CheckedAt = &now
	return out, nil
}

// lockRef serialises concurrent refreshes of the same image ref.
func (s *Service) lockRef(ref models.DockerImageRef) func() {
	key := refKey(ref.Image, ref.ImageTag)
	s.mu.Lock()
	m, ok := s.inflight[key]
	if !ok {
		m = &sync.Mutex{}
		s.inflight[key] = m
	}
	s.mu.Unlock()
	m.Lock()
	return m.Unlock
}

func (s *Service) cfgToken() string {
	if s.cfg == nil {
		return ""
	}
	return s.cfg.GitHubToken
}

// credentialsFor best-effort matches a stored RegistryCredential to an image's
// registry host. No match is not an error: the check is simply attempted
// anonymously, which succeeds for public images and fails with a recorded
// "identifiants manquants" message for private ones (that image then renders as
// "version inconnue" rather than breaking the whole sweep).
func (s *Service) credentialsFor(ctx context.Context, registry string, creds []models.RegistryCredential) (id, username, password string) {
	for _, c := range creds {
		if !sameRegistryHost(c.RegistryHost, registry) {
			continue
		}
		u, p, err := s.repo.GetRegistryCredentialAuth(ctx, c.ID)
		if err != nil || u == "" {
			return "", "", ""
		}
		return c.ID, u, p
	}
	return "", "", ""
}

// sameRegistryHost compares a user-entered registry_host ("ghcr.io",
// "https://ghcr.io/", "docker.io") with the host parsed out of an image ref.
func sameRegistryHost(configured, parsed string) bool {
	norm := func(h string) string {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
		h = strings.Trim(h, "/")
		switch h {
		case "docker.io", "index.docker.io", "registry.docker.io", "hub.docker.com":
			return "registry-1.docker.io"
		}
		return h
	}
	c, p := norm(configured), norm(parsed)
	return c != "" && c == p
}

// friendlyRegistryError turns a raw HTTP failure into something an operator can
// act on, preserving the pre-existing "latest introuvable" hint the tracker
// poller used to produce itself.
func friendlyRegistryError(err error, tag string, hadCreds bool) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	if tag == "latest" && strings.Contains(lower, "status 404") {
		return "tag latest introuvable pour cette image; utilisez un tag versionne (ex: v4, v4.4, v4.4.1)"
	}
	if !hadCreds && (strings.Contains(lower, "status 401") || strings.Contains(lower, "status 403")) {
		return "registre privé : aucun identifiant enregistré ne correspond à cet hôte de registre"
	}
	return msg
}

// ShouldResolveTag reports whether a tag is mutable/broad enough that the exact
// release behind its digest is worth resolving ("latest", "v5", "5.4"), as
// opposed to an already-pinned patch version ("v5.4.1") that is its own answer.
func ShouldResolveTag(tag string) bool {
	t := strings.TrimSpace(strings.ToLower(tag))
	if t == "" || t == "latest" {
		return true
	}
	t = strings.TrimPrefix(t, "v")
	parts := strings.Split(t, ".")
	if len(parts) != 1 && len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, ch := range p {
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
