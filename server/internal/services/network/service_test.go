package network

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	cfg   *models.NetworkTopologyConfig
	saved *models.NetworkTopologyConfig
}

func (f *fakeRepo) GetNetworkTopologyConfig(context.Context) (*models.NetworkTopologyConfig, error) {
	return f.cfg, nil
}
func (f *fakeRepo) SaveNetworkTopologyConfig(_ context.Context, cfg *models.NetworkTopologyConfig) error {
	f.saved = cfg
	return nil
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
	if err := NewService(repo, nil, nil).SaveTopologyConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SaveTopologyConfig: %v", err)
	}
	if repo.saved != cfg {
		t.Error("config not forwarded to the repository")
	}
}
