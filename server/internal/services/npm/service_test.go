package npm

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/npmclient"
)

// fakeRepo is an in-memory Repository fake covering only what these tests
// exercise; every method has a working implementation so the service's real
// control flow runs against it.
type fakeRepo struct {
	connections map[string]models.NPMConnection
	hosts       map[string]models.NPMProxyHost
	probes      map[string]bool // id -> enabled
	certs       map[string]bool // id -> enabled
	deletedConn string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		connections: map[string]models.NPMConnection{},
		hosts:       map[string]models.NPMProxyHost{},
		probes:      map[string]bool{},
		certs:       map[string]bool{},
	}
}

func (f *fakeRepo) CreateNPMConnection(context.Context, models.NPMConnectionRequest) (*models.NPMConnection, error) {
	return nil, nil
}
func (f *fakeRepo) ListNPMConnections(context.Context) ([]models.NPMConnection, error) {
	return nil, nil
}
func (f *fakeRepo) GetNPMConnectionByID(_ context.Context, id string) (*models.NPMConnection, error) {
	c := f.connections[id]
	return &c, nil
}
func (f *fakeRepo) GetEnabledNPMConnections(context.Context) ([]database.NPMConnectionFull, error) {
	return nil, nil
}
func (f *fakeRepo) GetNPMConnectionSecret(context.Context, string) (string, error) {
	return "secret", nil
}
func (f *fakeRepo) UpdateNPMConnection(_ context.Context, id string, _ models.NPMConnectionRequest) (*models.NPMConnection, error) {
	c := f.connections[id]
	return &c, nil
}
func (f *fakeRepo) DeleteNPMConnection(_ context.Context, id string) error {
	f.deletedConn = id
	delete(f.connections, id)
	for hid, h := range f.hosts {
		if h.ConnectionID == id {
			delete(f.hosts, hid)
		}
	}
	return nil
}
func (f *fakeRepo) UpdateNPMConnectionSuccess(context.Context, string) error       { return nil }
func (f *fakeRepo) UpdateNPMConnectionError(context.Context, string, string) error { return nil }

func (f *fakeRepo) UpsertNPMProxyHost(_ context.Context, h models.NPMProxyHost) (*models.NPMProxyHost, error) {
	for id, existing := range f.hosts {
		if existing.ConnectionID == h.ConnectionID && existing.NPMID == h.NPMID {
			existing.NPMEnabled = h.NPMEnabled
			existing.DomainNames = h.DomainNames
			f.hosts[id] = existing
			cp := existing
			return &cp, nil
		}
	}
	h.ID = h.ConnectionID + "-" + time.Now().Format("150405.000000000") + "-" + itoa(h.NPMID)
	f.hosts[h.ID] = h
	cp := h
	return &cp, nil
}
func (f *fakeRepo) ListNPMProxyHosts(_ context.Context, connectionID string) ([]models.NPMProxyHost, error) {
	var out []models.NPMProxyHost
	for _, h := range f.hosts {
		if h.ConnectionID == connectionID {
			out = append(out, h)
		}
	}
	return out, nil
}
func (f *fakeRepo) ListAllNPMProxyHostsEnriched(context.Context) ([]models.NPMProxyHostEnriched, error) {
	return nil, nil
}
func (f *fakeRepo) GetNPMProxyHostByID(_ context.Context, id string) (*models.NPMProxyHost, error) {
	h := f.hosts[id]
	return &h, nil
}
func (f *fakeRepo) UpdateNPMProxyHostLinks(_ context.Context, id string, probeID, certID *string) error {
	h := f.hosts[id]
	h.UptimeProbeID = probeID
	h.SSLCertificateID = certID
	f.hosts[id] = h
	return nil
}
func (f *fakeRepo) UpdateNPMProxyHostSettings(_ context.Context, id string, monitoring, uptime, ssl bool) error {
	h := f.hosts[id]
	h.MonitoringEnabled = monitoring
	h.UptimeMonitoringEnabled = uptime
	h.SSLMonitoringEnabled = ssl
	f.hosts[id] = h
	return nil
}
func (f *fakeRepo) UpdateNPMProxyHostNPMEnabled(_ context.Context, id string, enabled bool) error {
	h := f.hosts[id]
	h.NPMEnabled = enabled
	f.hosts[id] = h
	return nil
}
func (f *fakeRepo) GetNPMProxyHostsByConnectionNPMIDs(_ context.Context, connectionID string) (map[int]models.NPMProxyHost, error) {
	out := map[int]models.NPMProxyHost{}
	for _, h := range f.hosts {
		if h.ConnectionID == connectionID {
			out[h.NPMID] = h
		}
	}
	return out, nil
}
func (f *fakeRepo) RefreshNPMProxyHostSeen(context.Context, string, int, bool, time.Time) error {
	return nil
}

func (f *fakeRepo) CreateUptimeProbe(_ context.Context, p models.UptimeProbe) (*models.UptimeProbe, error) {
	p.ID = "probe-" + p.Name
	f.probes[p.ID] = true
	cp := p
	return &cp, nil
}
func (f *fakeRepo) CreateSSLCertificate(_ context.Context, c models.SSLCertificate) (*models.SSLCertificate, error) {
	c.ID = "cert-" + c.Name
	f.certs[c.ID] = true
	cp := c
	return &cp, nil
}
func (f *fakeRepo) SetUptimeProbeEnabled(_ context.Context, id string, enabled bool) error {
	f.probes[id] = enabled
	return nil
}
func (f *fakeRepo) SetSSLCertificateEnabled(_ context.Context, id string, enabled bool) error {
	f.certs[id] = enabled
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestDeleteConnection_DisablesLinkedMonitoring guards against the orphaned-cert
// regression: deleting a connection must disable every linked SSL cert / uptime
// probe, not just cascade-delete the npm_proxy_hosts rows (which SET NULL the
// link the other way, leaving the cert/probe enabled forever with no UI back to it).
func TestDeleteConnection_DisablesLinkedMonitoring(t *testing.T) {
	repo := newFakeRepo()
	repo.connections["conn1"] = models.NPMConnection{ID: "conn1"}
	repo.hosts["host1"] = models.NPMProxyHost{
		ID: "host1", ConnectionID: "conn1", NPMID: 1,
		UptimeProbeID: strPtr("probe-a"), SSLCertificateID: strPtr("cert-a"),
	}
	repo.probes["probe-a"] = true
	repo.certs["cert-a"] = true

	svc := NewService(repo)
	if err := svc.DeleteConnection(context.Background(), "conn1"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	if repo.probes["probe-a"] {
		t.Error("uptime probe should be disabled when its connection is deleted")
	}
	if repo.certs["cert-a"] {
		t.Error("SSL certificate should be disabled when its connection is deleted")
	}
	if repo.deletedConn != "conn1" {
		t.Error("connection itself should still be deleted")
	}
}

// TestRefreshSync_DisablesMonitoringForHostRemovedFromNPM guards against a host
// deleted from NPM outright (absent from the fetched list, not just Enabled=false)
// leaving its monitoring on forever, since the old loop only ever inspected hosts
// NPM still returned.
func TestRefreshSync_DisablesMonitoringForHostRemovedFromNPM(t *testing.T) {
	repo := newFakeRepo()
	repo.connections["conn1"] = models.NPMConnection{ID: "conn1", APIURL: "https://npm.local"}
	repo.hosts["host1"] = models.NPMProxyHost{
		ID: "host1", ConnectionID: "conn1", NPMID: 42,
		MonitoringEnabled: true, UptimeMonitoringEnabled: true,
		UptimeProbeID: strPtr("probe-a"),
	}
	repo.probes["probe-a"] = true

	svc := NewService(repo)
	svc.authFn = func(context.Context, string, string, string) (string, error) { return "tok", nil }
	// NPM no longer returns npm_id=42 at all (host deleted server-side in NPM).
	svc.listFn = func(context.Context, string, string) ([]npmclient.ProxyHost, error) {
		return []npmclient.ProxyHost{}, nil
	}

	if err := svc.RefreshSync(context.Background(), "conn1"); err != nil {
		t.Fatalf("RefreshSync: %v", err)
	}

	if repo.hosts["host1"].MonitoringEnabled {
		t.Error("monitoring should be cascaded off for a host no longer returned by NPM")
	}
	if repo.probes["probe-a"] {
		t.Error("linked uptime probe should be disabled once its host disappears from NPM")
	}
}

func strPtr(s string) *string { return &s }
