package alertrule

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	rule       *models.AlertRule
	getErr     error
	created    *models.AlertRule
	updated    *models.AlertRule
	deleted    bool
	hostExists bool
}

func (f *fakeRepo) ListAlertRulesAPI(context.Context) ([]models.AlertRule, error) { return nil, nil }
func (f *fakeRepo) GetAlertRuleByID(context.Context, int64) (*models.AlertRule, error) {
	return f.rule, f.getErr
}
func (f *fakeRepo) CreateAlertRule(_ context.Context, r *models.AlertRule) error {
	f.created = r
	return nil
}
func (f *fakeRepo) UpdateAlertRule(_ context.Context, r *models.AlertRule) error {
	f.updated = r
	return nil
}
func (f *fakeRepo) DeleteAlertRule(context.Context, int64) error     { f.deleted = true; return nil }
func (f *fakeRepo) HostExists(context.Context, string) (bool, error) { return f.hostExists, nil }
func (f *fakeRepo) DockerContainerExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeRepo) ComposeProjectExists(context.Context, string, string) (bool, error) {
	return true, nil
}
func (f *fakeRepo) ProxmoxConnectionExists(context.Context, string) (bool, error) { return true, nil }
func (f *fakeRepo) ProxmoxNodeExists(context.Context, string) (bool, error)       { return true, nil }
func (f *fakeRepo) ProxmoxStorageExists(context.Context, string) (bool, error)    { return true, nil }
func (f *fakeRepo) ProxmoxGuestExists(context.Context, string) (bool, error)      { return true, nil }
func (f *fakeRepo) ProxmoxDiskExists(context.Context, string) (bool, error)       { return true, nil }
func (f *fakeRepo) ResolveOpenAlertIncidentsByRule(context.Context, int64) (int64, error) {
	return 0, nil
}
func (f *fakeRepo) ResolveAlertIncident(context.Context, int64) error { return nil }
func (f *fakeRepo) GetAlertIncidents(context.Context, int, int) ([]models.AlertIncident, error) {
	return nil, nil
}
func (f *fakeRepo) GetHost(context.Context, string) (*models.Host, error) { return &models.Host{}, nil }
func (f *fakeRepo) GetDockerContainers(context.Context, string) ([]models.DockerContainer, error) {
	return nil, nil
}
func (f *fakeRepo) GetComposeProjectsByHost(context.Context, string) ([]models.ComposeProject, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertProxmoxConnections(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertProxmoxNodes(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertProxmoxStorages(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertProxmoxGuests(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertProxmoxDisks(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ListAlertDockerScopeHosts(context.Context) ([]models.AlertScopeOption, error) {
	return nil, nil
}
func (f *fakeRepo) ProxmoxConnectionName(context.Context, string) (string, error) { return "", nil }
func (f *fakeRepo) ProxmoxNodeLabelParts(context.Context, string) (string, string, error) {
	return "", "", nil
}
func (f *fakeRepo) ProxmoxStorageLabelParts(context.Context, string) (string, string, string, error) {
	return "", "", "", nil
}
func (f *fakeRepo) ProxmoxGuestLabelParts(context.Context, string) (string, string, string, string, int, error) {
	return "", "", "", "", 0, nil
}
func (f *fakeRepo) ProxmoxDiskLabelParts(context.Context, string) (string, string, string, string, error) {
	return "", "", "", "", nil
}

func newSvc(repo Repository) *Service { return NewService(repo, func(models.AlertRule) {}) }

func status(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.HTTPStatus
	}
	return 0
}

func TestValidateMetricOperator(t *testing.T) {
	svc := newSvc(&fakeRepo{})
	if status(svc.ValidateMetricOperator("cpu", "!!")) != 400 {
		t.Error("bad operator should be 400")
	}
	if status(svc.ValidateMetricOperator("nope", ">")) != 400 {
		t.Error("bad metric should be 400")
	}
	if err := svc.ValidateMetricOperator("cpu", ">"); err != nil {
		t.Errorf("valid pair should pass, got %v", err)
	}
}

func TestValidateActions(t *testing.T) {
	svc := newSvc(&fakeRepo{})
	if status(svc.ValidateActions(&models.AlertActions{Channels: []string{"carrier-pigeon"}})) != 400 {
		t.Error("unknown channel should be 400")
	}
	if status(svc.ValidateActions(&models.AlertActions{Cooldown: -1})) != 400 {
		t.Error("negative cooldown should be 400")
	}
	if err := svc.ValidateActions(&models.AlertActions{Channels: []string{"smtp", "browser"}}); err != nil {
		t.Errorf("valid channels should pass, got %v", err)
	}
}

func TestCreate_RejectsBadMetricBeforeDB(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).Create(context.Background(), models.AlertRuleCreate{
		Name: "x", Metric: "bogus", Operator: ">", SourceType: models.AlertSourceAgent,
	})
	if status(err) != 400 {
		t.Fatalf("bad metric should be 400, got %v", err)
	}
	if repo.created != nil {
		t.Error("must not hit the DB when validation fails")
	}
}

func TestUpdate_RejectsSourceTypeChange(t *testing.T) {
	repo := &fakeRepo{rule: &models.AlertRule{ID: 1, SourceType: models.AlertSourceAgent, Metric: "cpu", Operator: ">"}}
	st := models.AlertSourceProxmox
	err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{SourceType: &st})
	if status(err) != 400 {
		t.Fatalf("source_type change should be 400, got %v", err)
	}
	if repo.updated != nil {
		t.Error("must not persist on rejected source_type change")
	}
}

func TestGet_NotFound(t *testing.T) {
	if status(mustErr(newSvc(&fakeRepo{getErr: sql.ErrNoRows}).Get(context.Background(), 9))) != 404 {
		t.Error("missing rule should be 404")
	}
}

func TestDelete_NotFound(t *testing.T) {
	if status(newSvc(&fakeRepo{getErr: sql.ErrNoRows}).Delete(context.Background(), 9)) != 404 {
		t.Error("deleting a missing rule should be 404")
	}
}

func TestValidateDockerScope_MissingHost(t *testing.T) {
	svc := newSvc(&fakeRepo{hostExists: false})
	err := svc.ValidateDockerScope(context.Background(), &models.DockerMetricScope{HostID: "h1"})
	if status(err) != 400 {
		t.Errorf("missing host should be 400, got %v", err)
	}
}

func mustErr(_ *models.AlertRule, err error) error { return err }
