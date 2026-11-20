package network

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

type auditEntry struct{ username, action, ip, status string }

type fakeRepo struct {
	cfg     *models.NetworkTopologyConfig
	saved   *models.NetworkTopologyConfig
	saveErr error
	audits  []auditEntry
}

func (f *fakeRepo) GetNetworkTopologyConfig(context.Context) (*models.NetworkTopologyConfig, error) {
	return f.cfg, nil
}
func (f *fakeRepo) SaveNetworkTopologyConfig(_ context.Context, cfg *models.NetworkTopologyConfig) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = cfg
	return nil
}
func (f *fakeRepo) CreateAuditLog(_ context.Context, username, action, _, ip, _, status string) (int64, error) {
	f.audits = append(f.audits, auditEntry{username, action, ip, status})
	return 1, nil
}

func TestTopologySnapshot_MergesBaseAndConfig(t *testing.T) {
	cfg := &models.NetworkTopologyConfig{}
	repo := &fakeRepo{cfg: cfg}
	build := func(context.Context) (*models.NetworkSnapshot, error) {
		return &models.NetworkSnapshot{
			Hosts:      []models.NetworkHost{{}},
			Containers: []models.NetworkContainer{{}, {}},
		}, nil
	}
	snap, err := NewService(repo, build, nil).TopologySnapshot(context.Background())
	if err != nil {
		t.Fatalf("TopologySnapshot: %v", err)
	}
	if len(snap.Hosts) != 1 || len(snap.Containers) != 2 {
		t.Errorf("base data not carried: %d hosts, %d containers", len(snap.Hosts), len(snap.Containers))
	}
	if snap.Config != cfg {
		t.Error("persisted config not merged into the snapshot")
	}
	if snap.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be stamped")
	}
}

func TestTopologySnapshot_PropagatesBuildError(t *testing.T) {
	build := func(context.Context) (*models.NetworkSnapshot, error) {
		return nil, errors.New("boom")
	}
	if _, err := NewService(&fakeRepo{}, build, nil).TopologySnapshot(context.Background()); err == nil {
		t.Error("expected the builder error to propagate")
	}
}

func TestSaveTopologyConfig(t *testing.T) {
	repo := &fakeRepo{}
	cfg := &models.NetworkTopologyConfig{}
	if err := NewService(repo, nil, nil).SaveTopologyConfig(context.Background(), cfg, "alice", "10.0.0.1"); err != nil {
		t.Fatalf("SaveTopologyConfig: %v", err)
	}
	if repo.saved != cfg {
		t.Error("config not forwarded to the repository")
	}
	if len(repo.audits) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(repo.audits))
	}
	a := repo.audits[0]
	if a.username != "alice" || a.action != "update_network_config" || a.ip != "10.0.0.1" || a.status != "success" {
		t.Errorf("unexpected audit entry: %+v", a)
	}
}

func TestSaveTopologyConfig_AuditsFailure(t *testing.T) {
	repo := &fakeRepo{saveErr: errors.New("db down")}
	if err := NewService(repo, nil, nil).SaveTopologyConfig(context.Background(), &models.NetworkTopologyConfig{}, "alice", "10.0.0.1"); err == nil {
		t.Fatal("expected the repository error to propagate")
	}
	if len(repo.audits) != 1 || repo.audits[0].status != "failed" {
		t.Errorf("expected one failed audit entry, got %+v", repo.audits)
	}
}
