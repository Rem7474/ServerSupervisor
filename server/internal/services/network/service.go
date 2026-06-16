// Package network is the application/service layer for the network topology view.
// The live snapshot is built by the networkview package (which needs the concrete
// *database.DB), so it is injected as a builder func to keep the service decoupled;
// the persisted topology config goes through a Repository port.
package network

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port for the persisted topology config.
// *database.DB satisfies it structurally.
type Repository interface {
	GetNetworkTopologyConfig(ctx context.Context) (*models.NetworkTopologyConfig, error)
	SaveNetworkTopologyConfig(ctx context.Context, cfg *models.NetworkTopologyConfig) error
}

// SnapshotBuilder builds the live network snapshot (wired to networkview.BuildSnapshot).
type SnapshotBuilder func(ctx context.Context) (*models.NetworkSnapshot, error)

// Service holds the network use-cases.
type Service struct {
	repo  Repository
	build SnapshotBuilder
	bus   *events.Bus
}

func NewService(repo Repository, build SnapshotBuilder, bus *events.Bus) *Service {
	return &Service{repo: repo, build: build, bus: bus}
}

// Snapshot returns the live network snapshot.
func (s *Service) Snapshot(ctx context.Context) (*models.NetworkSnapshot, error) {
	return s.build(ctx)
}

// TopologyConfig returns the persisted topology configuration.
func (s *Service) TopologyConfig(ctx context.Context) (*models.NetworkTopologyConfig, error) {
	return s.repo.GetNetworkTopologyConfig(ctx)
}

// SaveTopologyConfig persists the topology configuration then wakes the network
// view subscribers (nil-safe when no bus is wired).
func (s *Service) SaveTopologyConfig(ctx context.Context, cfg *models.NetworkTopologyConfig) error {
	if err := s.repo.SaveNetworkTopologyConfig(ctx, cfg); err != nil {
		return err
	}
	s.bus.Publish(events.TopicNetwork)
	return nil
}

// TopologySnapshot returns the live snapshot merged with the persisted config.
func (s *Service) TopologySnapshot(ctx context.Context) (*models.TopologySnapshot, error) {
	base, err := s.build(ctx)
	if err != nil {
		return nil, err
	}
	config, _ := s.repo.GetNetworkTopologyConfig(ctx)
	return &models.TopologySnapshot{
		Hosts:      base.Hosts,
		Containers: base.Containers,
		Config:     config,
		UpdatedAt:  time.Now(),
	}, nil
}
