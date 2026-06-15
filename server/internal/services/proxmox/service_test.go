package proxmox

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// fakeRepo stubs the Repository; tests set only the fields they need.
type fakeRepo struct {
	link    *models.ProxmoxGuestLink
	created bool
}

func (f *fakeRepo) ListProxmoxConnections(context.Context) ([]models.ProxmoxConnection, error) {
	return nil, nil
}
func (f *fakeRepo) CreateProxmoxConnection(context.Context, string, string, string, string, bool, bool, int) (string, error) {
	f.created = true
	return "id", nil
}
func (f *fakeRepo) GetProxmoxConnectionByID(context.Context, string) (*models.ProxmoxConnection, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateProxmoxConnection(context.Context, string, string, string, string, string, bool, bool, int) error {
	return nil
}
func (f *fakeRepo) DeleteProxmoxConnection(context.Context, string) error { return nil }
func (f *fakeRepo) GetEnabledProxmoxConnections(context.Context) ([]database.ProxmoxConnectionFull, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxTokenSecret(context.Context, string) (string, error) { return "", nil }
func (f *fakeRepo) GetProxmoxSummary(context.Context) (models.ProxmoxSummary, error) {
	return models.ProxmoxSummary{}, nil
}
func (f *fakeRepo) ListProxmoxGuests(context.Context, string, string, string) ([]models.ProxmoxGuest, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxGuestsByNode(context.Context, string, string) ([]models.ProxmoxGuest, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxGuestMetricsSummary(context.Context, string, int, int) ([]models.ProxmoxNodeMetricsSummary, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxGuestLinks(context.Context, string) ([]models.ProxmoxGuestLink, error) {
	return nil, nil
}
func (f *fakeRepo) UpsertProxmoxGuestLink(_ context.Context, _, _, status, source string) (*models.ProxmoxGuestLink, error) {
	return &models.ProxmoxGuestLink{Status: status, MetricsSource: source}, nil
}
func (f *fakeRepo) GetProxmoxGuestLink(context.Context, string) (*models.ProxmoxGuestLink, error) {
	return f.link, nil
}
func (f *fakeRepo) UpdateProxmoxGuestLink(context.Context, string, *string, *string) (*models.ProxmoxGuestLink, error) {
	return f.link, nil
}
func (f *fakeRepo) DeleteProxmoxGuestLink(context.Context, string) error { return nil }
func (f *fakeRepo) GetProxmoxGuestLinkByGuest(context.Context, string) (*models.ProxmoxGuestLink, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxGuestLinkByHost(context.Context, string) (*models.ProxmoxGuestLink, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxLinkCandidates(context.Context, string) ([]models.ProxmoxGuest, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxNodes(context.Context) ([]models.ProxmoxNode, error) { return nil, nil }
func (f *fakeRepo) ListProxmoxNodesByConnection(context.Context, string) ([]models.ProxmoxNode, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxNode(context.Context, string) (*models.ProxmoxNode, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxNodeMetricsSummary(context.Context, int, int) ([]models.ProxmoxNodeMetricsSummary, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxNodeCPUTemperatureHistory(context.Context, string, int) ([]models.SystemMetrics, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxNodeFanRPMHistory(context.Context, string, int) ([]models.SystemMetrics, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxNodeCPUTempSourceCandidates(context.Context, string, string) ([]models.Host, error) {
	return nil, nil
}
func (f *fakeRepo) SetProxmoxNodeSensorSource(context.Context, string, string) error { return nil }
func (f *fakeRepo) BackfillProxmoxNodeSensorSources(context.Context) error           { return nil }
func (f *fakeRepo) GetHost(context.Context, string) (*models.Host, error)            { return nil, nil }
func (f *fakeRepo) ListProxmoxDisksByNode(context.Context, string, string) ([]models.ProxmoxDisk, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxDisksByHost(context.Context, string) ([]models.ProxmoxDisk, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxTasks(context.Context, string, int) ([]models.ProxmoxTask, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxTasksByNode(context.Context, string, string, int) ([]models.ProxmoxTask, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxBackupJobs(context.Context, string) ([]models.ProxmoxBackupJob, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxBackupRuns(context.Context, string) ([]models.ProxmoxBackupRun, error) {
	return nil, nil
}

func newSvc(repo Repository) *Service {
	return &Service{repo: repo, cfg: &config.Config{}, poller: nil}
}

func status(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.HTTPStatus
	}
	return 0
}

func TestParseVMID(t *testing.T) {
	cases := map[string]int{"100": 100, "1": 1, "0": 0, "-5": 0, "abc": 0, "": 0, "12x": 0}
	for in, want := range cases {
		if got := parseVMID(in); got != want {
			t.Errorf("parseVMID(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestCreateConnection_RequiresSecret(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).CreateConnection(context.Background(), models.ProxmoxConnectionRequest{Name: "x"})
	if status(err) != 400 {
		t.Fatalf("missing token_secret should be 400, got %v", err)
	}
	if repo.created {
		t.Error("must not create the connection without a secret")
	}
}

func TestGetConnection_NotFound(t *testing.T) {
	// GetProxmoxConnectionByID returns (nil, nil) -> not found.
	if status(mustConnErr(newSvc(&fakeRepo{}).GetConnection(context.Background(), "x"))) != 404 {
		t.Error("missing connection should be 404")
	}
}

func TestUpdateLink_InvalidStatus(t *testing.T) {
	repo := &fakeRepo{link: &models.ProxmoxGuestLink{}}
	bad := "bogus"
	_, err := newSvc(repo).UpdateLink(context.Background(), "id", models.ProxmoxGuestLinkUpdate{Status: &bad})
	if status(err) != 400 {
		t.Fatalf("invalid status should be 400, got %v", err)
	}
}

func TestUpdateLink_NotFound(t *testing.T) {
	// GetProxmoxGuestLink returns nil -> not found.
	if status(mustLinkErr(newSvc(&fakeRepo{}).UpdateLink(context.Background(), "id", models.ProxmoxGuestLinkUpdate{}))) != 404 {
		t.Error("missing link should be 404")
	}
}

func TestNodeServiceAction_InvalidAction(t *testing.T) {
	_, err := newSvc(&fakeRepo{}).NodeServiceAction(context.Background(), "node", "pveproxy", "frobnicate")
	if status(err) != 400 {
		t.Fatalf("invalid action should be 400, got %v", err)
	}
}

func mustConnErr(_ *models.ProxmoxConnection, err error) error { return err }
func mustLinkErr(_ *models.ProxmoxGuestLink, err error) error  { return err }
