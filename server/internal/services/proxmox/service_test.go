package proxmox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// fakeRepo stubs the Repository; tests set only the fields they need.
type fakeRepo struct {
	link    *models.ProxmoxGuestLink
	created bool

	// Used by AllGuestNetworks tests only; nil/empty for every other test,
	// which preserves their existing nil-returning behavior below.
	nodes        []models.ProxmoxNode
	enabledConns []database.ProxmoxConnectionFull
	connByID     map[string]*models.ProxmoxConnection
	guestsByNode map[string][]models.ProxmoxGuest
	guestByID    map[string]*models.ProxmoxGuest
	consoleCreds map[string][2]string // connID -> [username, password]
}

func (f *fakeRepo) ListProxmoxConnections(context.Context) ([]models.ProxmoxConnection, error) {
	return nil, nil
}
func (f *fakeRepo) CreateProxmoxConnection(context.Context, models.ProxmoxConnectionRequest) (string, error) {
	f.created = true
	return "id", nil
}
func (f *fakeRepo) GetProxmoxConnectionByID(_ context.Context, id string) (*models.ProxmoxConnection, error) {
	if f.connByID != nil {
		return f.connByID[id], nil
	}
	return nil, nil
}
func (f *fakeRepo) UpdateProxmoxConnection(context.Context, string, models.ProxmoxConnectionRequest) error {
	return nil
}
func (f *fakeRepo) DeleteProxmoxConnection(context.Context, string) error { return nil }
func (f *fakeRepo) GetEnabledProxmoxConnections(context.Context) ([]database.ProxmoxConnectionFull, error) {
	return f.enabledConns, nil
}
func (f *fakeRepo) GetProxmoxTokenSecret(context.Context, string) (string, error) { return "", nil }
func (f *fakeRepo) GetProxmoxConsoleCredentials(_ context.Context, id string) (string, string, error) {
	if creds, ok := f.consoleCreds[id]; ok {
		return creds[0], creds[1], nil
	}
	return "", "", nil
}
func (f *fakeRepo) GetProxmoxSummary(context.Context) (models.ProxmoxSummary, error) {
	return models.ProxmoxSummary{}, nil
}
func (f *fakeRepo) ListProxmoxGuests(context.Context, string, string, string) ([]models.ProxmoxGuest, error) {
	return nil, nil
}
func (f *fakeRepo) ListProxmoxGuestsByNode(_ context.Context, connectionID, nodeName string) ([]models.ProxmoxGuest, error) {
	if f.guestsByNode != nil {
		return f.guestsByNode[connectionID+"|"+nodeName], nil
	}
	return nil, nil
}
func (f *fakeRepo) GetProxmoxGuestByID(_ context.Context, id string) (*models.ProxmoxGuest, error) {
	if g, ok := f.guestByID[id]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
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
func (f *fakeRepo) ListProxmoxNodes(context.Context) ([]models.ProxmoxNode, error) {
	return f.nodes, nil
}
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
func (f *fakeRepo) GetExposureByIPs(context.Context, []string, time.Time) (map[string]*models.HostExposure, error) {
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

func TestGuestAction_InvalidAction(t *testing.T) {
	_, err := newSvc(&fakeRepo{}).GuestAction(context.Background(), "guest-1", "frobnicate")
	if status(err) != 400 {
		t.Fatalf("invalid action should be 400, got %v", err)
	}
}

// stop (hard power-off, no ACPI shutdown) is deliberately excluded from the
// whitelist — see the "Proxmox integration" note in the root CLAUDE.md.
func TestGuestAction_ExcludesHardStop(t *testing.T) {
	_, err := newSvc(&fakeRepo{}).GuestAction(context.Background(), "guest-1", "stop")
	if status(err) != 400 {
		t.Fatalf("hard stop must not be whitelisted, got %v", err)
	}
}

func TestGuestAction_GuestNotFound(t *testing.T) {
	// GetProxmoxGuestByID's fakeRepo stub always errors -> not found.
	_, err := newSvc(&fakeRepo{}).GuestAction(context.Background(), "missing", "start")
	if status(err) != 404 {
		t.Fatalf("missing guest should be 404, got %v", err)
	}
}

func mustConnErr(_ *models.ProxmoxConnection, err error) error { return err }
func mustLinkErr(_ *models.ProxmoxGuestLink, err error) error  { return err }

func TestOpenGuestConsole_GuestNotFound(t *testing.T) {
	// GetProxmoxGuestByID's fakeRepo stub always errors -> not found.
	_, _, err := newSvc(&fakeRepo{}).OpenGuestConsole(context.Background(), "missing")
	if status(err) != 404 {
		t.Fatalf("missing guest should be 404, got %v", err)
	}
}

func TestOpenGuestConsole_RejectsQemu(t *testing.T) {
	repo := &fakeRepo{guestByID: map[string]*models.ProxmoxGuest{
		"g1": {ID: "g1", GuestType: "vm", ConnectionID: "c1", NodeName: "pve1", VMID: 100},
	}}
	_, _, err := newSvc(repo).OpenGuestConsole(context.Background(), "g1")
	if status(err) != 400 {
		t.Fatalf("qemu guest console should be 400 (V1 is LXC-only), got %v", err)
	}
}

func TestOpenGuestConsole_MissingCredentials(t *testing.T) {
	repo := &fakeRepo{
		guestByID: map[string]*models.ProxmoxGuest{
			"g1": {ID: "g1", GuestType: "lxc", ConnectionID: "c1", NodeName: "pve1", VMID: 101},
		},
		enabledConns: []database.ProxmoxConnectionFull{{
			ProxmoxConnection: models.ProxmoxConnection{ID: "c1"},
			TokenSecret:       "secret",
		}},
		connByID: map[string]*models.ProxmoxConnection{
			"c1": {ID: "c1", APIURL: "https://pve.invalid:8006", TokenID: "user@pve!token"},
		},
		// consoleCreds intentionally left empty for c1.
	}
	_, _, err := newSvc(repo).OpenGuestConsole(context.Background(), "g1")
	if status(err) != 400 {
		t.Fatalf("missing console credentials should be 400, got %v", err)
	}
}

// fakePVEServer answers the two REST calls TestConnection can make: the
// plain API reachability check (GET /nodes) and, when console credentials
// are supplied, the login used to validate them (POST /access/ticket).
func fakePVEServer(t *testing.T, loginOK bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		if !loginOK {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:X=="}}`))
	})
	return httptest.NewServer(mux)
}

func TestTestConnection_NoConsoleCredentials(t *testing.T) {
	srv := fakePVEServer(t, true)
	defer srv.Close()

	result := newSvc(&fakeRepo{}).TestConnection(srv.URL, "user@pve!token", "secret", false, "", "")
	if !result.Success {
		t.Fatalf("expected API test to succeed, got error %q", result.Error)
	}
	if result.ConsoleConfigured {
		t.Error("ConsoleConfigured should be false with no pve_username/pve_password")
	}
	if result.ConsoleOK {
		t.Error("ConsoleOK should be false when console isn't configured")
	}
}

func TestTestConnection_ConsoleCredentialsValid(t *testing.T) {
	srv := fakePVEServer(t, true)
	defer srv.Close()

	result := newSvc(&fakeRepo{}).TestConnection(srv.URL, "user@pve!token", "secret", false, "root@pam", "hunter2")
	if !result.Success {
		t.Fatalf("expected API test to succeed, got error %q", result.Error)
	}
	if !result.ConsoleConfigured || !result.ConsoleOK {
		t.Errorf("expected console configured+ok, got %+v", result)
	}
	if result.ConsoleError != "" {
		t.Errorf("ConsoleError should be empty on success, got %q", result.ConsoleError)
	}
}

func TestTestConnection_ConsoleCredentialsInvalid(t *testing.T) {
	srv := fakePVEServer(t, false)
	defer srv.Close()

	result := newSvc(&fakeRepo{}).TestConnection(srv.URL, "user@pve!token", "secret", false, "root@pam", "wrong")
	if !result.Success {
		t.Fatalf("expected API test to succeed (token auth unaffected by console login), got error %q", result.Error)
	}
	if !result.ConsoleConfigured {
		t.Error("ConsoleConfigured should be true when credentials are supplied")
	}
	if result.ConsoleOK {
		t.Error("ConsoleOK should be false when PVE login fails")
	}
	if result.ConsoleError == "" {
		t.Error("expected a ConsoleError message when PVE login fails")
	}
}

func TestTestConnection_APIFailure(t *testing.T) {
	result := newSvc(&fakeRepo{}).TestConnection("http://127.0.0.1:1", "user@pve!token", "secret", false, "", "")
	if result.Success {
		t.Error("expected API test to fail against an unreachable host")
	}
	if result.Error == "" {
		t.Error("expected an Error message on API failure")
	}
}

func TestTestConnectionByID_NotFound(t *testing.T) {
	_, err := newSvc(&fakeRepo{}).TestConnectionByID(context.Background(), "missing")
	if status(err) != 404 {
		t.Fatalf("missing connection should be 404, got %v", err)
	}
}

func TestTestConnectionByID_UsesStoredCredentials(t *testing.T) {
	srv := fakePVEServer(t, true)
	defer srv.Close()

	repo := &fakeRepo{
		connByID: map[string]*models.ProxmoxConnection{
			"c1": {ID: "c1", APIURL: srv.URL, TokenID: "user@pve!token"},
		},
		consoleCreds: map[string][2]string{"c1": {"root@pam", "hunter2"}},
	}
	result, err := newSvc(repo).TestConnectionByID(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success || !result.ConsoleConfigured || !result.ConsoleOK {
		t.Errorf("expected a fully successful test, got %+v", result)
	}
}
