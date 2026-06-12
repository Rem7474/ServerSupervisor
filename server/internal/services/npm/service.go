// Package npm is the application/service layer for the Nginx Proxy Manager
// integration. It owns the business logic for testing connections, previewing
// proxy hosts, and importing selected hosts as uptime probes + SSL certificates.
package npm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/npmclient"
)

// Repository is the data-access port the service depends on. *database.DB
// satisfies it structurally; tests provide an in-memory fake.
type Repository interface {
	CreateNPMConnection(ctx context.Context, req models.NPMConnectionRequest) (*models.NPMConnection, error)
	ListNPMConnections(ctx context.Context) ([]models.NPMConnection, error)
	GetNPMConnectionByID(ctx context.Context, id string) (*models.NPMConnection, error)
	GetEnabledNPMConnections(ctx context.Context) ([]database.NPMConnectionFull, error)
	GetNPMConnectionSecret(ctx context.Context, id string) (string, error)
	UpdateNPMConnection(ctx context.Context, id string, req models.NPMConnectionRequest) (*models.NPMConnection, error)
	DeleteNPMConnection(ctx context.Context, id string) error
	UpdateNPMConnectionSuccess(ctx context.Context, id string) error
	UpdateNPMConnectionError(ctx context.Context, id string, errMsg string) error

	UpsertNPMProxyHost(ctx context.Context, h models.NPMProxyHost) (*models.NPMProxyHost, error)
	ListNPMProxyHosts(ctx context.Context, connectionID string) ([]models.NPMProxyHost, error)
	ListAllNPMProxyHostsEnriched(ctx context.Context) ([]models.NPMProxyHostEnriched, error)
	GetNPMProxyHostByID(ctx context.Context, id string) (*models.NPMProxyHost, error)
	UpdateNPMProxyHostLinks(ctx context.Context, id string, probeID, certID *string) error
	UpdateNPMProxyHostSettings(ctx context.Context, id string, monitoring, uptime, ssl bool) error
	GetNPMProxyHostsByConnectionNPMIDs(ctx context.Context, connectionID string) (map[int]models.NPMProxyHost, error)
	RefreshNPMProxyHostSeen(ctx context.Context, connectionID string, npmID int, npmEnabled bool, lastSeenAt time.Time) error

	CreateUptimeProbe(ctx context.Context, p models.UptimeProbe) (*models.UptimeProbe, error)
	CreateSSLCertificate(ctx context.Context, c models.SSLCertificate) (*models.SSLCertificate, error)
	SetUptimeProbeEnabled(ctx context.Context, id string, enabled bool) error
	SetSSLCertificateEnabled(ctx context.Context, id string, enabled bool) error
}

// AuthFn authenticates against an NPM instance and returns a Bearer token.
// Injected so tests can avoid real network I/O.
type AuthFn func(ctx context.Context, apiURL, identity, secret string) (string, error)

// ListFn fetches proxy hosts from an NPM instance using a Bearer token.
type ListFn func(ctx context.Context, apiURL, token string) ([]npmclient.ProxyHost, error)

// Service holds the NPM use-cases.
type Service struct {
	repo   Repository
	authFn AuthFn
	listFn ListFn
}

// NewService wires the service with the production NPM client.
func NewService(repo Repository) *Service {
	return &Service{
		repo:   repo,
		authFn: npmclient.Authenticate,
		listFn: npmclient.GetProxyHosts,
	}
}

// ─── Connection CRUD ─────────────────────────────────────────────────────────

func (s *Service) CreateConnection(ctx context.Context, req models.NPMConnectionRequest) (*models.NPMConnection, error) {
	if req.Secret == "" {
		return nil, apperr.Validation("secret is required when creating a connection")
	}
	return s.repo.CreateNPMConnection(ctx, req)
}

func (s *Service) ListConnections(ctx context.Context) ([]models.NPMConnection, error) {
	conns, err := s.repo.ListNPMConnections(ctx)
	if err != nil {
		return nil, err
	}
	if conns == nil {
		conns = []models.NPMConnection{}
	}
	return conns, nil
}

func (s *Service) GetConnection(ctx context.Context, id string) (*models.NPMConnection, error) {
	c, err := s.repo.GetNPMConnectionByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("npm connection not found")
	}
	return c, err
}

func (s *Service) UpdateConnection(ctx context.Context, id string, req models.NPMConnectionRequest) (*models.NPMConnection, error) {
	if _, err := s.GetConnection(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.UpdateNPMConnection(ctx, id, req)
}

func (s *Service) DeleteConnection(ctx context.Context, id string) error {
	if _, err := s.GetConnection(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteNPMConnection(ctx, id)
}

// ─── Test ────────────────────────────────────────────────────────────────────

// TestConnection authenticates against NPM and lists proxy hosts without
// modifying any DB state. Returns nil on success.
func (s *Service) TestConnection(ctx context.Context, apiURL, identity, secret string) error {
	token, err := s.authFn(ctx, apiURL, identity, secret)
	if err != nil {
		return err
	}
	_, err = s.listFn(ctx, apiURL, token)
	return err
}

// ─── Preview ─────────────────────────────────────────────────────────────────

// PreviewProxyHosts fetches live proxy hosts from NPM and enriches each with its
// import status from ServerSupervisor. Nothing is written to the DB.
func (s *Service) PreviewProxyHosts(ctx context.Context, connectionID string) ([]models.NPMProxyHostPreview, error) {
	conn, err := s.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	secret, err := s.repo.GetNPMConnectionSecret(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("retrieve npm secret: %w", err)
	}

	token, err := s.authFn(ctx, conn.APIURL, conn.Identity, secret)
	if err != nil {
		return nil, fmt.Errorf("NPM authenticate: %w", err)
	}
	hosts, err := s.listFn(ctx, conn.APIURL, token)
	if err != nil {
		return nil, fmt.Errorf("NPM list proxy-hosts: %w", err)
	}

	existing, err := s.repo.GetNPMProxyHostsByConnectionNPMIDs(ctx, connectionID)
	if err != nil {
		return nil, err
	}

	previews := make([]models.NPMProxyHostPreview, 0, len(hosts))
	for _, h := range hosts {
		p := models.NPMProxyHostPreview{
			NPMID:       h.ID,
			DomainNames: h.DomainNames,
			ForwardHost: h.ForwardHost,
			ForwardPort: h.ForwardPort,
			SSLEnabled:  h.SSLEnabled(),
			NPMEnabled:  h.Enabled,
		}
		if stored, found := existing[h.ID]; found {
			p.AlreadyImported = true
			p.UptimeProbeID = stored.UptimeProbeID
			p.SSLCertificateID = stored.SSLCertificateID
		}
		previews = append(previews, p)
	}
	return previews, nil
}

// ─── Import ──────────────────────────────────────────────────────────────────

// ImportSelectedProxyHosts imports the proxy hosts identified by npmIDs from the
// given connection. For each one it creates an uptime probe and (when SSL is
// enabled) an SSL certificate, then links them in npm_proxy_hosts.
// Returns the count of newly imported hosts.
func (s *Service) ImportSelectedProxyHosts(ctx context.Context, connectionID string, npmIDs []int) (int, error) {
	conn, err := s.GetConnection(ctx, connectionID)
	if err != nil {
		return 0, err
	}
	secret, err := s.repo.GetNPMConnectionSecret(ctx, connectionID)
	if err != nil {
		return 0, fmt.Errorf("retrieve npm secret: %w", err)
	}

	token, err := s.authFn(ctx, conn.APIURL, conn.Identity, secret)
	if err != nil {
		return 0, fmt.Errorf("NPM authenticate: %w", err)
	}
	hosts, err := s.listFn(ctx, conn.APIURL, token)
	if err != nil {
		return 0, fmt.Errorf("NPM list proxy-hosts: %w", err)
	}

	// Build a set for fast lookup.
	wanted := make(map[int]struct{}, len(npmIDs))
	for _, id := range npmIDs {
		wanted[id] = struct{}{}
	}

	existing, err := s.repo.GetNPMProxyHostsByConnectionNPMIDs(ctx, connectionID)
	if err != nil {
		return 0, err
	}

	imported := 0
	for _, h := range hosts {
		if _, ok := wanted[h.ID]; !ok {
			continue
		}

		sslEnabled := h.SSLEnabled()
		record := models.NPMProxyHost{
			ConnectionID: connectionID,
			NPMID:        h.ID,
			DomainNames:  h.DomainNames,
			ForwardHost:  h.ForwardHost,
			ForwardPort:  h.ForwardPort,
			SSLEnabled:   sslEnabled,
			NPMEnabled:   h.Enabled,
		}

		stored, err := s.repo.UpsertNPMProxyHost(ctx, record)
		if err != nil {
			return imported, fmt.Errorf("upsert proxy host %d: %w", h.ID, err)
		}

		// Skip linking if already fully linked (idempotent re-import).
		if _, alreadyIn := existing[h.ID]; alreadyIn && stored.UptimeProbeID != nil {
			continue
		}

		primaryDomain := primaryDomain(h.DomainNames)
		var probeID, certID *string

		// Create uptime probe.
		scheme := "http"
		if sslEnabled {
			scheme = "https"
		}
		probeTarget := scheme + "://" + primaryDomain
		followRedirects := true
		verifyTLS := true
		probe, err := s.repo.CreateUptimeProbe(ctx, models.UptimeProbe{
			Name:            primaryDomain,
			Type:            "http",
			Target:          probeTarget,
			IntervalSec:     60,
			TimeoutSec:      10,
			ExpectedStatus:  200,
			FollowRedirects: followRedirects,
			VerifyTLS:       verifyTLS,
			Enabled:         true,
		})
		if err != nil {
			return imported, fmt.Errorf("create uptime probe for %s: %w", primaryDomain, err)
		}
		probeID = &probe.ID

		// Create SSL certificate when SSL is active.
		if sslEnabled {
			cert, err := s.repo.CreateSSLCertificate(ctx, models.SSLCertificate{
				Name:    primaryDomain,
				Host:    primaryDomain,
				Port:    443,
				Enabled: true,
			})
			if err != nil {
				return imported, fmt.Errorf("create ssl cert for %s: %w", primaryDomain, err)
			}
			certID = &cert.ID
		}

		if err := s.repo.UpdateNPMProxyHostLinks(ctx, stored.ID, probeID, certID); err != nil {
			return imported, fmt.Errorf("link npm proxy host %s: %w", stored.ID, err)
		}
		imported++
	}

	if err := s.repo.UpdateNPMConnectionSuccess(ctx, connectionID); err != nil {
		return imported, err
	}
	return imported, nil
}

// ─── Global proxy host list ──────────────────────────────────────────────────

// ListAllProxyHosts returns every imported proxy host across all connections,
// enriched with connection name and live uptime/SSL status.
func (s *Service) ListAllProxyHosts(ctx context.Context) ([]models.NPMProxyHostEnriched, error) {
	hosts, err := s.repo.ListAllNPMProxyHostsEnriched(ctx)
	if err != nil {
		return nil, err
	}
	if hosts == nil {
		hosts = []models.NPMProxyHostEnriched{}
	}
	return hosts, nil
}

// ─── Monitoring toggles ───────────────────────────────────────────────────────

// UpdateProxyHostMonitoring applies the monitoring toggle changes from req to
// the proxy host identified by id, then propagates enable/disable to the linked
// uptime probe and SSL certificate.
func (s *Service) UpdateProxyHostMonitoring(ctx context.Context, id string, req models.NPMProxyHostUpdateRequest) (*models.NPMProxyHostEnriched, error) {
	host, err := s.repo.GetNPMProxyHostByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("npm proxy host not found")
	}
	if err != nil {
		return nil, err
	}

	monitoring := host.MonitoringEnabled
	uptime := host.UptimeMonitoringEnabled
	ssl := host.SSLMonitoringEnabled

	if req.MonitoringEnabled != nil {
		monitoring = *req.MonitoringEnabled
		if !monitoring {
			uptime = false
			ssl = false
		}
	}
	if req.UptimeMonitoringEnabled != nil {
		uptime = *req.UptimeMonitoringEnabled
	}
	if req.SSLMonitoringEnabled != nil {
		ssl = *req.SSLMonitoringEnabled
	}
	// Recalculate master flag: true when at least one sub-flag is active.
	if req.MonitoringEnabled == nil {
		monitoring = uptime || ssl
	}

	if err := s.repo.UpdateNPMProxyHostSettings(ctx, id, monitoring, uptime, ssl); err != nil {
		return nil, err
	}

	if host.UptimeProbeID != nil {
		_ = s.repo.SetUptimeProbeEnabled(ctx, *host.UptimeProbeID, monitoring && uptime)
	}
	if host.SSLCertificateID != nil {
		_ = s.repo.SetSSLCertificateEnabled(ctx, *host.SSLCertificateID, monitoring && ssl)
	}

	hosts, err := s.repo.ListAllNPMProxyHostsEnriched(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range hosts {
		if e.ID == id {
			return &e, nil
		}
	}
	// Fallback — shouldn't happen, but return a minimal enriched result.
	return &models.NPMProxyHostEnriched{NPMProxyHost: *host}, nil
}

// ─── Background refresh ───────────────────────────────────────────────────────

// RefreshSync updates last_seen_at and npm_enabled for already-imported hosts.
// When npm_enabled transitions to false for a monitoring-enabled host, it cascades
// the disable to linked uptime probe and SSL certificate.
// It never auto-imports new hosts; those appear in PreviewProxyHosts for manual selection.
func (s *Service) RefreshSync(ctx context.Context, connectionID string) error {
	conn, err := s.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}
	secret, err := s.repo.GetNPMConnectionSecret(ctx, connectionID)
	if err != nil {
		return err
	}

	token, err := s.authFn(ctx, conn.APIURL, conn.Identity, secret)
	if err != nil {
		_ = s.repo.UpdateNPMConnectionError(ctx, connectionID, err.Error())
		return err
	}
	hosts, err := s.listFn(ctx, conn.APIURL, token)
	if err != nil {
		_ = s.repo.UpdateNPMConnectionError(ctx, connectionID, err.Error())
		return err
	}

	// Index existing DB state so we can detect npm_enabled transitions.
	existing, _ := s.repo.GetNPMProxyHostsByConnectionNPMIDs(ctx, connectionID)

	now := time.Now()
	for _, h := range hosts {
		_ = s.repo.RefreshNPMProxyHostSeen(ctx, connectionID, h.ID, h.Enabled, now)

		// Auto-disable monitoring when NPM reports the proxy host as disabled.
		if stored, ok := existing[h.ID]; ok && !h.Enabled && stored.MonitoringEnabled {
			falseVal := false
			_, _ = s.UpdateProxyHostMonitoring(ctx, stored.ID, models.NPMProxyHostUpdateRequest{
				MonitoringEnabled: &falseVal,
			})
		}
	}
	return s.repo.UpdateNPMConnectionSuccess(ctx, connectionID)
}

// RefreshAllEnabled calls RefreshSync for every enabled connection whose
// last_success_at is older than poll_interval_sec (or has never run).
// Called periodically by the background poller.
func (s *Service) RefreshAllEnabled(ctx context.Context) {
	conns, err := s.repo.GetEnabledNPMConnections(ctx)
	if err != nil {
		return
	}
	for _, conn := range conns {
		if conn.LastSuccessAt != nil && time.Since(*conn.LastSuccessAt) < time.Duration(conn.PollIntervalSec)*time.Second {
			continue
		}
		_ = s.RefreshSync(ctx, conn.ID)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// primaryDomain returns the first domain without scheme/port.
func primaryDomain(domains []string) string {
	if len(domains) == 0 {
		return ""
	}
	d := domains[0]
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	return strings.TrimRight(d, "/")
}

// ListProxyHosts returns all already-imported proxy hosts for a connection.
func (s *Service) ListProxyHosts(ctx context.Context, connectionID string) ([]models.NPMProxyHost, error) {
	hosts, err := s.repo.ListNPMProxyHosts(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if hosts == nil {
		hosts = []models.NPMProxyHost{}
	}
	return hosts, nil
}
