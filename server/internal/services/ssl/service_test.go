package ssl

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	created      *models.SSLCertificate
	checkResult  *models.SSLCertificate
	stored       *models.SSLCertificate
	getErr       error
	getCallCount int
}

func (f *fakeRepo) ListSSLCertificates(context.Context) ([]models.SSLCertificate, error) {
	return nil, nil
}
func (f *fakeRepo) GetSSLCertificate(context.Context, string) (*models.SSLCertificate, error) {
	f.getCallCount++
	return f.stored, f.getErr
}
func (f *fakeRepo) CreateSSLCertificate(_ context.Context, c models.SSLCertificate) (*models.SSLCertificate, error) {
	cp := c
	f.created = &cp
	return &cp, nil
}
func (f *fakeRepo) UpdateSSLCertificate(context.Context, models.SSLCertificate) error { return nil }
func (f *fakeRepo) DeleteSSLCertificate(context.Context, string) error               { return nil }
func (f *fakeRepo) UpdateSSLCertificateCheckResult(_ context.Context, c models.SSLCertificate) error {
	cp := c
	f.checkResult = &cp
	return nil
}

func boolPtr(b bool) *bool { return &b }

func TestCreateCert_AppliesDefaults(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.CreateCert(context.Background(), models.SSLCertificateRequest{
		Name: "  site  ", Host: "  example.com  ",
	}); err != nil {
		t.Fatalf("CreateCert: %v", err)
	}
	got := repo.created
	if got.Name != "site" || got.Host != "example.com" {
		t.Errorf("name/host should be trimmed, got %q / %q", got.Name, got.Host)
	}
	if got.Port != 443 {
		t.Errorf("port 0 should default to 443, got %d", got.Port)
	}
	if !got.Enabled {
		t.Error("enabled should default to true")
	}
}

func TestCreateCert_EnabledOverride(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, _ = svc.CreateCert(context.Background(), models.SSLCertificateRequest{
		Name: "x", Host: "x", Port: 8443, Enabled: boolPtr(false),
	})
	if repo.created.Enabled {
		t.Error("explicit enabled=false must override the default")
	}
	if repo.created.Port != 8443 {
		t.Errorf("explicit port must be kept, got %d", repo.created.Port)
	}
}

func TestGetCert_NotFoundMapsToAppErr(t *testing.T) {
	svc := NewService(&fakeRepo{getErr: sql.ErrNoRows})
	_, err := svc.GetCert(context.Background(), "missing")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("expected apperr 404, got %v", err)
	}
}

// TestCheckNow_RunsChecksAndPersists verifies orchestration: fetch -> run the
// injected checker -> persist the observed result -> return the refreshed record.
func TestCheckNow_RunsChecksAndPersists(t *testing.T) {
	repo := &fakeRepo{stored: &models.SSLCertificate{ID: "c1", Host: "example.com"}}
	svc := NewService(repo)
	issuer := "Let's Encrypt"
	svc.check = func(_ context.Context, c models.SSLCertificate) models.SSLCertificate {
		c.Issuer = issuer
		return c
	}
	if _, err := svc.CheckNow(context.Background(), "c1"); err != nil {
		t.Fatalf("CheckNow: %v", err)
	}
	if repo.checkResult == nil || repo.checkResult.Issuer != issuer {
		t.Errorf("CheckNow must persist the checker's observed result, got %+v", repo.checkResult)
	}
}

func TestListCerts_NeverNil(t *testing.T) {
	got, err := NewService(&fakeRepo{}).ListCerts(context.Background())
	if err != nil {
		t.Fatalf("ListCerts: %v", err)
	}
	if got == nil {
		t.Error("ListCerts must return a non-nil slice")
	}
}
