// Package ssl is the application/service layer for SSL/TLS certificate monitoring.
// It owns the certificate business logic (defaults, check-now orchestration)
// behind a Repository port, so the logic is unit-testable without a database and
// the HTTP handler is reduced to request/response translation.
package ssl

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/synthetic"
)

// Repository is the data-access port the service depends on. *database.DB
// satisfies it structurally; tests provide an in-memory fake.
type Repository interface {
	ListSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error)
	GetSSLCertificate(ctx context.Context, id string) (*models.SSLCertificate, error)
	CreateSSLCertificate(ctx context.Context, c models.SSLCertificate) (*models.SSLCertificate, error)
	UpdateSSLCertificate(ctx context.Context, c models.SSLCertificate) error
	DeleteSSLCertificate(ctx context.Context, id string) error
	UpdateSSLCertificateCheckResult(ctx context.Context, c models.SSLCertificate) error
}

// CertChecker performs a TLS handshake and returns the certificate with its
// freshly observed expiry/issuer/etc. Injected so tests avoid real network I/O;
// defaults to synthetic.CheckCertificate.
type CertChecker func(ctx context.Context, c models.SSLCertificate) models.SSLCertificate

// Service holds the SSL-certificate use-cases.
type Service struct {
	repo  Repository
	check CertChecker
}

// NewService wires the service with the production certificate checker.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, check: synthetic.CheckCertificate}
}

// certFromRequest maps a create/update request onto a certificate model, applying
// the server-side defaults (the business rules this layer owns).
func certFromRequest(p models.SSLCertificateRequest) models.SSLCertificate {
	m := models.SSLCertificate{
		Name:       strings.TrimSpace(p.Name),
		Host:       strings.TrimSpace(p.Host),
		Port:       p.Port,
		ServerName: strings.TrimSpace(p.ServerName),
		Enabled:    true,
	}
	if m.Port == 0 {
		m.Port = 443
	}
	if p.Enabled != nil {
		m.Enabled = *p.Enabled
	}
	return m
}

// ListCerts returns all monitored certificates (never nil).
func (s *Service) ListCerts(ctx context.Context) ([]models.SSLCertificate, error) {
	certs, err := s.repo.ListSSLCertificates(ctx)
	if err != nil {
		return nil, err
	}
	if certs == nil {
		certs = []models.SSLCertificate{}
	}
	return certs, nil
}

// GetCert returns one certificate, or apperr.NotFound when it is absent.
func (s *Service) GetCert(ctx context.Context, id string) (*models.SSLCertificate, error) {
	cert, err := s.repo.GetSSLCertificate(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("certificate not found")
	}
	if err != nil {
		return nil, err
	}
	return cert, nil
}

// CreateCert validates+defaults the request and persists a new certificate.
func (s *Service) CreateCert(ctx context.Context, req models.SSLCertificateRequest) (*models.SSLCertificate, error) {
	return s.repo.CreateSSLCertificate(ctx, certFromRequest(req))
}

// UpdateCert applies the request to the certificate identified by id and returns
// the stored result.
func (s *Service) UpdateCert(ctx context.Context, id string, req models.SSLCertificateRequest) (*models.SSLCertificate, error) {
	m := certFromRequest(req)
	m.ID = id
	if err := s.repo.UpdateSSLCertificate(ctx, m); err != nil {
		return nil, err
	}
	return s.GetCert(ctx, id)
}

// DeleteCert removes a certificate by id.
func (s *Service) DeleteCert(ctx context.Context, id string) error {
	return s.repo.DeleteSSLCertificate(ctx, id)
}

// CheckNow runs a TLS handshake immediately, persists the observed result and
// returns the refreshed record.
func (s *Service) CheckNow(ctx context.Context, id string) (*models.SSLCertificate, error) {
	cert, err := s.GetCert(ctx, id)
	if err != nil {
		return nil, err
	}
	updated := s.check(ctx, *cert)
	if err := s.repo.UpdateSSLCertificateCheckResult(ctx, updated); err != nil {
		return nil, err
	}
	return s.GetCert(ctx, updated.ID)
}
