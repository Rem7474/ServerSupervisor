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
	UpdateNPMProxyHostNPMEnabled(ctx context.Context, id string, enabled bool) error
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

// UpdateProxyHostMonitoring applies monitoring toggle changes and creates uptime
// probes / SSL certificates on demand the first time a resource is enabled.
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

	switch {
	case req.MonitoringEnabled != nil && !*req.MonitoringEnabled:
		// Master OFF → disable everything.
		monitoring, uptime, ssl = false, false, false
	case req.MonitoringEnabled != nil && *req.MonitoringEnabled:
		// Master ON → enable both sub-flags (user can refine afterwards).
		monitoring, uptime, ssl = true, true, true
	default:
		// Individual sub-flag change.
		if req.UptimeMonitoringEnabled != nil {
			uptime = *req.UptimeMonitoringEnabled
		}
		if req.SSLMonitoringEnabled != nil {
			ssl = *req.SSLMonitoringEnabled
		}
		monitoring = uptime || ssl
	}

	// Create uptime probe on first activation.
	if monitoring && uptime && host.UptimeProbeID == nil {
		if probe, err := s.createProbe(ctx, host); err == nil {
			_ = s.repo.UpdateNPMProxyHostLinks(ctx, id, &probe.ID, host.SSLCertificateID)
			host.UptimeProbeID = &probe.ID
		}
	}

	// Create SSL certificate on first activation (only when host has SSL).
	if monitoring && ssl && host.SSLCertificateID == nil && host.SSLEnabled {
		if cert, err := s.createCert(ctx, host); err == nil {
			_ = s.repo.UpdateNPMProxyHostLinks(ctx, id, host.UptimeProbeID, &cert.ID)
			host.SSLCertificateID = &cert.ID
		}
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

	all, err := s.repo.ListAllNPMProxyHostsEnriched(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == id {
			return &e, nil
		}
	}
	return &models.NPMProxyHostEnriched{NPMProxyHost: *host}, nil
}

// SetNPMProxyHostEnabled enables or disables a proxy host in NPM via its API,
// then updates the local DB state. Disabling cascades monitoring off.
func (s *Service) SetNPMProxyHostEnabled(ctx context.Context, proxyHostID string, enable bool) (*models.NPMProxyHostEnriched, error) {
	host, err := s.repo.GetNPMProxyHostByID(ctx, proxyHostID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("npm proxy host not found")
	}
	if err != nil {
		return nil, err
	}

	conn, err := s.GetConnection(ctx, host.ConnectionID)
	if err != nil {
		return nil, err
	}
	secret, err := s.repo.GetNPMConnectionSecret(ctx, host.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("retrieve npm secret: %w", err)
	}
	token, err := s.authFn(ctx, conn.APIURL, conn.Identity, secret)
	if err != nil {
		return nil, fmt.Errorf("NPM authenticate: %w", err)
	}

	if enable {
		if err := npmclient.EnableProxyHost(ctx, conn.APIURL, token, host.NPMID); err != nil {
			return nil, err
		}
	} else {
		if err := npmclient.DisableProxyHost(ctx, conn.APIURL, token, host.NPMID); err != nil {
			return nil, err
		}
	}

	_ = s.repo.UpdateNPMProxyHostNPMEnabled(ctx, proxyHostID, enable)

	// Disabling in NPM cascades monitoring off.
	if !enable && host.MonitoringEnabled {
		falseVal := false
		_, _ = s.UpdateProxyHostMonitoring(ctx, proxyHostID, models.NPMProxyHostUpdateRequest{
			MonitoringEnabled: &falseVal,
		})
	}

	all, err := s.repo.ListAllNPMProxyHostsEnriched(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.ID == proxyHostID {
			return &e, nil
		}
	}
	return &models.NPMProxyHostEnriched{NPMProxyHost: *host}, nil
}

func (s *Service) createProbe(ctx context.Context, host *models.NPMProxyHost) (*models.UptimeProbe, error) {
	domain := primaryDomain(host.DomainNames)
	scheme := "http"
	if host.SSLEnabled {
		scheme = "https"
	}
	return s.repo.CreateUptimeProbe(ctx, models.UptimeProbe{
		Name:            domain,
		Type:            "http",
		Target:          scheme + "://" + domain,
		IntervalSec:     60,
		TimeoutSec:      10,
		ExpectedStatus:  200,
		FollowRedirects: true,
		VerifyTLS:       true,
		Enabled:         true,
	})
}

func (s *Service) createCert(ctx context.Context, host *models.NPMProxyHost) (*models.SSLCertificate, error) {
	domain := primaryDomain(host.DomainNames)
	return s.repo.CreateSSLCertificate(ctx, models.SSLCertificate{
		Name:    domain,
		Host:    domain,
		Port:    443,
		Enabled: true,
	})
}

// ─── Background refresh ───────────────────────────────────────────────────────

// RefreshSync fetches all proxy hosts from NPM and upserts them into the DB.
// New hosts are added with monitoring disabled; existing hosts keep their monitoring state.
// When NPM reports a host as disabled, monitoring is cascaded off automatically.
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

	falseVal := false
	for _, h := range hosts {
		stored, err := s.repo.UpsertNPMProxyHost(ctx, models.NPMProxyHost{
			ConnectionID: connectionID,
			NPMID:        h.ID,
			DomainNames:  h.DomainNames,
			ForwardHost:  h.ForwardHost,
			ForwardPort:  h.ForwardPort,
			SSLEnabled:   h.SSLEnabled(),
			NPMEnabled:   h.Enabled,
		})
		if err != nil {
			continue
		}
		// Auto-disable monitoring when NPM reports the host as disabled.
		if !h.Enabled && stored.MonitoringEnabled {
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
