package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// The SNMP credential used in these tests is a placeholder: it never has
// the shape of a real secret.
const testCommunity = "test-placeholder"

type fakeAgentRepo struct {
	hosts       map[string]*models.Host
	all         []models.Host
	allErr      error
	commands    map[string]*models.RemoteCommand
	cancelOK    bool
	cancelErr   error
	cancelCalls int
	listErr     error
	audits      []string
}

func (f *fakeAgentRepo) GetHost(_ context.Context, id string) (*models.Host, error) {
	if h, ok := f.hosts[id]; ok {
		return h, nil
	}
	return nil, errors.New("no rows")
}

func (f *fakeAgentRepo) GetAllHosts(context.Context) ([]models.Host, error) { return f.all, f.allErr }

func (f *fakeAgentRepo) GetRemoteCommandByID(_ context.Context, id string) (*models.RemoteCommand, error) {
	if c, ok := f.commands[id]; ok {
		return c, nil
	}
	return nil, errors.New("no rows")
}

func (f *fakeAgentRepo) GetRemoteCommandsByHostAndModule(_ context.Context, hostID, module string, _ int) ([]models.RemoteCommand, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []models.RemoteCommand
	for _, c := range f.commands {
		if c.HostID == hostID && c.Module == module {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (f *fakeAgentRepo) CancelPendingRemoteCommand(_ context.Context, id, output string) (bool, error) {
	f.cancelCalls++
	if f.cancelErr != nil {
		return false, f.cancelErr
	}
	if f.cancelOK {
		c := f.commands[id]
		c.Status, c.Output = "failed", output
	}
	return f.cancelOK, nil
}

func (f *fakeAgentRepo) CreateAuditLog(_ context.Context, _, action, _, _, details, _ string) (int64, error) {
	f.audits = append(f.audits, action+" "+details)
	return 1, nil
}

type fakeDispatcher struct {
	reqs []dispatch.Request
	err  error
}

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.reqs = append(f.reqs, req)
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-1", HostID: req.HostID, Module: req.Module, Action: req.Action, Payload: req.Payload, Status: "pending"}}, nil
}

type fakeStatus struct{ calls []string }

func (f *fakeStatus) BroadcastStatus(id, status, output string) {
	f.calls = append(f.calls, id+" "+status+" "+output)
}

func fixture(collectors map[string]bool) (*AgentService, *fakeAgentRepo, *fakeDispatcher, *fakeStatus) {
	repo := &fakeAgentRepo{
		hosts:    map[string]*models.Host{"h1": {ID: "h1", Collectors: collectors}},
		commands: map[string]*models.RemoteCommand{},
	}
	disp := &fakeDispatcher{}
	st := &fakeStatus{}
	return NewAgentService(repo, disp, &config.Config{}, st), repo, disp, st
}

var fullAgent = map[string]bool{"network_discovery": true, "network_scan": true}

var admin = Actor{Username: "admin", IPAddress: "10.0.0.5"}

func i18nKey(t *testing.T, err error) string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	return ae.I18nKey
}

func TestLimits(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	l := s.Limits()
	if l.MaxPorts != defaultMaxPorts || l.MinPrefixLen != 24 || len(l.DefaultPorts) != len(DefaultPorts) || len(l.AllowedSubnets) != 0 {
		t.Fatalf("limits = %+v", l)
	}
	s.cfg.DiscoveryPortScanMaxPorts = 99999
	s.cfg.DiscoveryAllowedSubnets = []string{"192.168.1.7/24", "garbage", "fd00::/64"}
	l = s.Limits()
	if l.MaxPorts != hardMaxPorts || len(l.AllowedSubnets) != 1 || l.AllowedSubnets[0] != "192.168.1.0/24" {
		t.Fatalf("limits = %+v", l)
	}
	s.cfg.DiscoveryPortScanMaxPorts = 100
	if s.Limits().MaxPorts != 100 {
		t.Fatal("configured max ignored")
	}
}

func TestValidateScanCIDR(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	cases := map[string]string{
		"nope":           apperr.CodeDiscoveryCIDRInvalid,
		"fd00::/120":     apperr.CodeDiscoveryIPv4Only,
		"192.168.0.0/23": apperr.CodeDiscoveryPrefixRange,
		"8.8.8.0/24":     apperr.CodeDiscoveryPrivateOnly,
		"100.64.0.0/24":  apperr.CodeDiscoveryPrivateOnly,
	}
	for cidr, want := range cases {
		_, err := s.ValidateScanCIDR(cidr)
		if got := i18nKey(t, err); got != want {
			t.Errorf("%s: key %s, want %s", cidr, got, want)
		}
	}
	p, err := s.ValidateScanCIDR(" 172.16.4.9/24 ")
	if err != nil || p.String() != "172.16.4.0/24" {
		t.Fatalf("got %v %v", p, err)
	}

	s.cfg.DiscoveryAllowedSubnets = []string{"10.1.0.0/16"}
	if _, err := s.ValidateScanCIDR("10.1.2.0/24"); err != nil {
		t.Fatalf("inside allowed subnet: %v", err)
	}
	_, err = s.ValidateScanCIDR("10.2.0.0/24")
	if i18nKey(t, err) != apperr.CodeDiscoverySubnetNotAllowed {
		t.Fatalf("outside allowed subnet: %v", err)
	}
}

// A configured list whose entries are all invalid must not fall back to
// "any private range": it allows nothing.
func TestAllowedSubnetsFailClosed(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	s.cfg.DiscoveryAllowedSubnets = []string{"10.0.0.0/8 192.168.0.0/16", "192.168.1.1", "lan"}
	for _, cidr := range []string{"10.0.0.0/24", "192.168.1.0/24"} {
		if _, err := s.ValidateScanCIDR(cidr); i18nKey(t, err) != apperr.CodeDiscoverySubnetNotAllowed {
			t.Errorf("%s: %v", cidr, err)
		}
	}
	if _, err := s.validateTarget("192.168.1.1"); i18nKey(t, err) != apperr.CodeDiscoverySubnetNotAllowed {
		t.Errorf("snmp target: %v", err)
	}
	s.cfg.DiscoveryAllowedSubnets = nil
	if _, err := s.ValidateScanCIDR("10.0.0.0/24"); err != nil {
		t.Fatalf("no list configured: %v", err)
	}
}

// One discovery job at a time per agent: a pending or running job blocks
// the next start with a 409, a finished one does not, another agent is not
// affected.
func TestOneActiveJobPerAgent(t *testing.T) {
	s, repo, disp, _ := fixture(fullAgent)
	repo.hosts["h2"] = &models.Host{ID: "h2", Collectors: fullAgent}
	for _, status := range []string{"pending", "running"} {
		repo.commands["busy"] = &models.RemoteCommand{ID: "busy", HostID: "h1", Module: "discovery", Action: "port_scan", Status: status}
		_, err := s.StartARP(context.Background(), admin, "h1")
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.HTTPStatus != 409 || ae.I18nKey != apperr.CodeDiscoveryJobActive || ae.Params["job_id"] != "busy" {
			t.Fatalf("%s job: %v", status, err)
		}
	}
	if len(disp.reqs) != 0 {
		t.Fatal("a job was dispatched while another one is active")
	}
	// The SNMP credentials of a refused probe are not kept.
	_, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Community: testCommunity})
	if i18nKey(t, err) != apperr.CodeDiscoveryJobActive || len(s.secrets) != 0 {
		t.Fatalf("snmp refused: %v, %d secrets held", err, len(s.secrets))
	}
	if _, err := s.StartARP(context.Background(), admin, "h2"); err != nil {
		t.Fatalf("other agent: %v", err)
	}
	repo.commands["busy"].Status = "completed"
	repo.commands["other"] = &models.RemoteCommand{ID: "other", HostID: "h1", Module: "docker", Status: "running"}
	if _, err := s.StartARP(context.Background(), admin, "h1"); err != nil {
		t.Fatalf("finished job / other module: %v", err)
	}
	repo.listErr = errors.New("db down")
	if _, err := s.StartARP(context.Background(), admin, "h1"); err == nil {
		t.Fatal("lookup error swallowed")
	}
}

func TestParsePorts(t *testing.T) {
	ports, err := ParsePorts("", 10)
	if err != nil || len(ports) != len(DefaultPorts) {
		t.Fatalf("defaults: %v %v", ports, err)
	}
	ports, err = ParsePorts(" 443, 22,80-82,, 22 ", 10)
	if err != nil || len(ports) != 5 || ports[0] != 22 || ports[4] != 443 {
		t.Fatalf("parsed %v %v", ports, err)
	}
	for _, bad := range []string{"0", "65536", "a", "90-80", "1-x", ",", "-5"} {
		if _, err := ParsePorts(bad, 100); i18nKey(t, err) != apperr.CodeDiscoveryPortsInvalid {
			t.Errorf("%q: %v", bad, err)
		}
	}
	for _, big := range []string{"1-11", "1-6,7-12"} {
		if _, err := ParsePorts(big, 10); i18nKey(t, err) != apperr.CodeDiscoveryTooManyPorts {
			t.Errorf("%q: %v", big, err)
		}
	}
}

func TestStartARP(t *testing.T) {
	s, _, disp, _ := fixture(map[string]bool{"network_discovery": true})
	id, err := s.StartARP(context.Background(), admin, "h1")
	if err != nil || id != "cmd-1" {
		t.Fatalf("%s %v", id, err)
	}
	r := disp.reqs[0]
	if r.Module != "discovery" || r.Action != "arp" || r.Audit == nil || r.Audit.Action != "discovery_arp" || r.TriggeredBy != "admin" {
		t.Fatalf("request = %+v", r)
	}

	if _, err := s.StartARP(context.Background(), admin, "missing"); err == nil {
		t.Fatal("unknown host accepted")
	}
	s.repo.(*fakeAgentRepo).hosts["old"] = &models.Host{ID: "old"}
	if _, err := s.StartARP(context.Background(), admin, "old"); i18nKey(t, err) != apperr.CodeDiscoveryAgentUnsupported {
		t.Fatalf("old agent: %v", err)
	}
	disp.err = errors.New("db down")
	if _, err := s.StartARP(context.Background(), admin, "h1"); err == nil {
		t.Fatal("dispatch error swallowed")
	}
}

func TestStartPortScan(t *testing.T) {
	s, repo, disp, _ := fixture(fullAgent)
	_, err := s.StartPortScan(context.Background(), admin, models.PortScanRequest{HostID: "h1", CIDR: "192.168.1.0/24", Ports: "22,80"})
	if err != nil {
		t.Fatal(err)
	}
	r := disp.reqs[0]
	if r.Action != "port_scan" || r.Target != "192.168.1.0/24" || r.Payload != `{"cidr":"192.168.1.0/24","concurrency":64,"ports":[22,80],"rate_per_second":500,"timeout_ms":800}` {
		t.Fatalf("request = %+v", r)
	}
	if r.Audit.Details != `{"cidr":"192.168.1.0/24","ports":2}` {
		t.Fatalf("audit = %s", r.Audit.Details)
	}

	if _, err := s.StartPortScan(context.Background(), admin, models.PortScanRequest{HostID: "h1", CIDR: "8.8.8.0/24"}); i18nKey(t, err) != apperr.CodeDiscoveryPrivateOnly {
		t.Fatalf("public: %v", err)
	}
	if _, err := s.StartPortScan(context.Background(), admin, models.PortScanRequest{HostID: "h1", CIDR: "10.0.0.0/24", Ports: "x"}); i18nKey(t, err) != apperr.CodeDiscoveryPortsInvalid {
		t.Fatalf("ports: %v", err)
	}
	repo.hosts["h1"].Collectors = map[string]bool{"network_discovery": true}
	if _, err := s.StartPortScan(context.Background(), admin, models.PortScanRequest{HostID: "h1", CIDR: "10.0.0.0/24"}); i18nKey(t, err) != apperr.CodeDiscoveryScanDisabled {
		t.Fatalf("opt-out agent: %v", err)
	}
	if len(disp.reqs) != 1 {
		t.Fatal("refused scans must not be dispatched")
	}
}

func TestStartSNMPKeepsSecretsOutOfPayloadAndAudit(t *testing.T) {
	s, _, disp, _ := fixture(fullAgent)
	_, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "192.168.1.9", Version: "2c", Community: testCommunity, IncludeInterfaces: true})
	if err != nil {
		t.Fatal(err)
	}
	r := disp.reqs[0]
	if strings.Contains(r.Payload, testCommunity) || strings.Contains(r.Audit.Details, testCommunity) {
		t.Fatalf("secret leaked: %s / %s", r.Payload, r.Audit.Details)
	}
	var payload map[string]any
	_ = json.Unmarshal([]byte(r.Payload), &payload)
	ref, _ := payload["secret_ref"].(string)
	if len(ref) != 32 || payload["port"] != float64(161) || payload["target"] != "192.168.1.9" {
		t.Fatalf("payload = %v", payload)
	}

	// Claim: the credentials are injected once, then forgotten.
	cmds := []models.PendingCommand{
		{ID: "other", Module: "docker", Action: "logs", Payload: r.Payload},
		{ID: "bad", Module: "discovery", Action: "snmp", Payload: "not json"},
		{ID: "cmd-1", Module: "discovery", Action: "snmp", Payload: r.Payload},
	}
	out := s.PrepareClaimed(context.Background(), "h1", cmds)
	if out[0].Payload != r.Payload || out[1].Payload != "not json" {
		t.Fatal("unrelated commands modified")
	}
	if !strings.Contains(out[2].Payload, `"secrets":{"community":"`+testCommunity+`"}`) || strings.Contains(out[2].Payload, "secret_ref") {
		t.Fatalf("claimed payload = %s", out[2].Payload)
	}
	again := s.PrepareClaimed(context.Background(), "h1", []models.PendingCommand{{Module: "discovery", Action: "snmp", Payload: r.Payload}})
	if strings.Contains(again[0].Payload, "secrets") {
		t.Fatal("credentials served twice")
	}
}

func TestSNMPSecretExpires(t *testing.T) {
	s, _, disp, _ := fixture(fullAgent)
	now := time.Now()
	s.now = func() time.Time { return now }
	if _, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Community: testCommunity}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(secretTTL + time.Minute)
	out := s.PrepareClaimed(context.Background(), "h1", []models.PendingCommand{{Module: "discovery", Action: "snmp", Payload: disp.reqs[0].Payload}})
	if strings.Contains(out[0].Payload, "secrets") {
		t.Fatal("expired credentials injected")
	}
}

func TestStartSNMPDispatchFailureDropsSecret(t *testing.T) {
	s, _, disp, _ := fixture(fullAgent)
	disp.err = errors.New("db down")
	if _, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Community: testCommunity}); err == nil {
		t.Fatal("expected error")
	}
	if len(s.secrets) != 0 {
		t.Fatal("secret kept after a failed dispatch")
	}
}

func TestStartSNMPValidation(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	long := strings.Repeat("x", 256)
	pass := "abcdefgh"
	v3 := func(mut func(*models.SNMPProbeRequest)) models.SNMPProbeRequest {
		r := models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "3", Username: "mon", SecurityLevel: "authPriv", AuthProtocol: "SHA256", AuthPassphrase: pass, PrivProtocol: "AES", PrivPassphrase: pass}
		mut(&r)
		return r
	}
	cases := map[string]struct {
		req   models.SNMPProbeRequest
		field string
	}{
		"version":       {models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "1"}, "version"},
		"port":          {models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Port: 70000}, "port"},
		"no community":  {models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c"}, "community"},
		"long communty": {models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Community: long}, "community"},
		"username":      {v3(func(r *models.SNMPProbeRequest) { r.Username = "" }), "username"},
		"level":         {v3(func(r *models.SNMPProbeRequest) { r.SecurityLevel = "bogus" }), "security_level"},
		"auth proto":    {v3(func(r *models.SNMPProbeRequest) { r.AuthProtocol = "MD5" }), "auth_protocol"},
		"auth pass":     {v3(func(r *models.SNMPProbeRequest) { r.AuthPassphrase = "short" }), "auth_passphrase"},
		"priv proto":    {v3(func(r *models.SNMPProbeRequest) { r.PrivProtocol = "DES" }), "priv_protocol"},
		"priv pass":     {v3(func(r *models.SNMPProbeRequest) { r.PrivPassphrase = long }), "priv_passphrase"},
	}
	for name, c := range cases {
		_, err := s.StartSNMP(context.Background(), admin, c.req)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.I18nKey != apperr.CodeDiscoverySNMPInvalid || ae.Params["field"] != c.field {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, target := range []string{"nope", "fd00::1"} {
		if _, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: target}); i18nKey(t, err) != apperr.CodeDiscoveryIPInvalid {
			t.Errorf("%s: %v", target, err)
		}
	}
	if _, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "1.1.1.1"}); i18nKey(t, err) != apperr.CodeDiscoveryPrivateOnly {
		t.Errorf("public target: %v", err)
	}

	// Valid v3 levels.
	for _, r := range []models.SNMPProbeRequest{
		v3(func(*models.SNMPProbeRequest) {}),
		v3(func(r *models.SNMPProbeRequest) { r.SecurityLevel = "authNoPriv"; r.PrivProtocol = "" }),
		v3(func(r *models.SNMPProbeRequest) { r.SecurityLevel = "noAuthNoPriv" }),
	} {
		if _, err := s.StartSNMP(context.Background(), admin, r); err != nil {
			t.Errorf("%s: %v", r.SecurityLevel, err)
		}
	}
}

func TestCancelPendingJob(t *testing.T) {
	s, repo, _, st := fixture(fullAgent)
	repo.commands["c1"] = &models.RemoteCommand{ID: "c1", HostID: "h1", Module: "discovery", Action: "port_scan", Status: "pending"}
	repo.cancelOK = true
	job, err := s.Cancel(context.Background(), admin, "c1")
	if err != nil || job.Status != "failed" || job.Error != "cancelled" {
		t.Fatalf("job = %+v, %v", job, err)
	}
	if len(st.calls) != 1 || st.calls[0] != "c1 failed cancelled" || len(repo.audits) != 1 || !strings.HasPrefix(repo.audits[0], "discovery_cancel") {
		t.Fatalf("broadcast %v audits %v", st.calls, repo.audits)
	}
	if s.CancelRequested("c1") {
		t.Fatal("a pending cancel needs no agent signal")
	}
}

// Cancelling an SNMP probe no agent claimed yet drops its credentials.
func TestCancelPendingSNMPForgetsSecret(t *testing.T) {
	s, repo, disp, _ := fixture(fullAgent)
	if _, err := s.StartSNMP(context.Background(), admin, models.SNMPProbeRequest{HostID: "h1", Target: "10.0.0.2", Version: "2c", Community: testCommunity}); err != nil {
		t.Fatal(err)
	}
	if len(s.secrets) != 1 {
		t.Fatalf("secrets held = %d", len(s.secrets))
	}
	repo.commands["cmd-1"] = &models.RemoteCommand{ID: "cmd-1", HostID: "h1", Module: "discovery", Action: "snmp", Status: "pending", Payload: disp.reqs[0].Payload}
	repo.cancelOK = true
	if _, err := s.Cancel(context.Background(), admin, "cmd-1"); err != nil {
		t.Fatal(err)
	}
	if len(s.secrets) != 0 {
		t.Fatal("credentials of a cancelled probe are still held")
	}
	s.forgetSecret("not json") // tolerated
}

// The sweeper drops expired credentials without waiting for SNMP activity.
func TestSweeperDropsExpiredSecrets(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	now := time.Now()
	s.now = func() time.Time { return now }
	s.holdSecret("ref", snmpSecrets{Community: testCommunity})
	s.Sweep()
	if len(s.secrets) != 1 {
		t.Fatal("a live secret was swept")
	}
	now = now.Add(secretTTL + time.Second)
	s.Sweep()
	if len(s.secrets) != 0 {
		t.Fatal("expired secret kept")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.RunSweeper(ctx); close(done) }()
	cancel()
	<-done
}

func TestCancelPendingClaimedMeanwhile(t *testing.T) {
	s, repo, _, st := fixture(fullAgent)
	repo.commands["c1"] = &models.RemoteCommand{ID: "c1", Module: "discovery", Action: "port_scan", Status: "pending"}
	if _, err := s.Cancel(context.Background(), admin, "c1"); err != nil {
		t.Fatal(err)
	}
	if !s.CancelRequested("c1") || len(st.calls) != 0 {
		t.Fatal("a command claimed during the cancel must be stopped on the agent")
	}
}

func TestCancelRunningAndFinishedJobs(t *testing.T) {
	s, repo, _, _ := fixture(fullAgent)
	repo.commands["run"] = &models.RemoteCommand{ID: "run", Module: "discovery", Action: "port_scan", Status: "running"}
	repo.commands["done"] = &models.RemoteCommand{ID: "done", Module: "discovery", Action: "snmp", Status: "completed", Output: `{"target":"10.0.0.2"}`}
	repo.commands["docker"] = &models.RemoteCommand{ID: "docker", Module: "docker", Status: "running"}

	if _, err := s.Cancel(context.Background(), admin, "run"); err != nil || !s.CancelRequested("run") {
		t.Fatalf("running cancel: %v", err)
	}
	s.HandleCommandCompletion("run", "completed")
	if s.CancelRequested("run") {
		t.Fatal("cancel flag kept after completion")
	}
	if job, err := s.Cancel(context.Background(), admin, "done"); err != nil || job.Status != "completed" || repo.cancelCalls != 0 {
		t.Fatalf("finished job: %+v %v", job, err)
	}
	if len(repo.audits) != 1 {
		t.Fatalf("audits = %v", repo.audits)
	}
	if _, err := s.Cancel(context.Background(), admin, "docker"); err == nil {
		t.Fatal("non-discovery command cancelled")
	}
	repo.cancelErr = errors.New("db down")
	repo.commands["p"] = &models.RemoteCommand{ID: "p", Module: "discovery", Status: "pending"}
	if _, err := s.Cancel(context.Background(), admin, "p"); err == nil {
		t.Fatal("db error swallowed")
	}
}

func TestCancelFlagExpires(t *testing.T) {
	s, _, _, _ := fixture(fullAgent)
	now := time.Now()
	s.now = func() time.Time { return now }
	s.markCancelled("old")
	now = now.Add(cancelTTL + time.Minute)
	s.markCancelled("new")
	if s.CancelRequested("old") || !s.CancelRequested("new") {
		t.Fatal("stale cancel requests are not swept")
	}
}

func TestJobResults(t *testing.T) {
	s, repo, _, _ := fixture(fullAgent)
	repo.all = []models.Host{{ID: "h9", Name: "nas", IPAddress: "192.168.1.9"}, {ID: "nip"}}
	repo.commands["arp"] = &models.RemoteCommand{ID: "arp", Module: "discovery", Action: "arp", Status: "completed",
		Output: `{"entries":[{"ip":"192.168.1.9","mac":"BC:24:11:00:00:01","interface":"eth0"},{"ip":"192.168.1.20","mac":"02:42:ac:11:00:02","interface":"docker0"},{"ip":"192.168.1.30","mac":"a6:00:00:00:00:01","interface":"eth0"}]}`}
	repo.commands["arp-empty"] = &models.RemoteCommand{ID: "arp-empty", Module: "discovery", Action: "arp", Status: "completed", Output: `{}`}
	repo.commands["scan"] = &models.RemoteCommand{ID: "scan", Module: "discovery", Action: "port_scan", Status: "completed",
		Output: `{"cidr":"192.168.1.0/24","cancelled":true,"truncated":true,"hosts":[{"ip":"192.168.1.9","open_ports":[22]},{"ip":"192.168.1.50","open_ports":[80]}]}`}
	repo.commands["scan-empty"] = &models.RemoteCommand{ID: "scan-empty", Module: "discovery", Action: "port_scan", Status: "completed", Output: `{}`}
	repo.commands["snmp"] = &models.RemoteCommand{ID: "snmp", Module: "discovery", Action: "snmp", Status: "completed", Output: `{"target":"192.168.1.9","sys_name":"nas"}`}
	repo.commands["failed"] = &models.RemoteCommand{ID: "failed", Module: "discovery", Action: "snmp", Status: "failed", Output: "ERROR: SNMP request failed: timeout"}
	repo.commands["garbage"] = &models.RemoteCommand{ID: "garbage", Module: "discovery", Action: "snmp", Status: "completed", Output: "not json"}
	repo.commands["weird"] = &models.RemoteCommand{ID: "weird", Module: "discovery", Action: "other", Status: "completed", Output: "{}"}
	repo.commands["running"] = &models.RemoteCommand{ID: "running", Module: "discovery", Action: "port_scan", Status: "running"}

	job, err := s.Job(context.Background(), "arp")
	if err != nil {
		t.Fatal(err)
	}
	e := job.ARP.Entries
	if !e[0].AlreadyRegistered || e[0].ExistingHostName != "nas" || e[0].VendorHint != "Proxmox VE" || e[0].LocallyAdministered {
		t.Fatalf("entry 0 = %+v", e[0])
	}
	if e[1].VendorHint != "Docker" || !e[1].LocallyAdministered || e[1].AlreadyRegistered {
		t.Fatalf("entry 1 = %+v", e[1])
	}
	if e[2].VendorHint != "" || !e[2].LocallyAdministered {
		t.Fatalf("entry 2 = %+v", e[2])
	}
	if job, _ := s.Job(context.Background(), "arp-empty"); job.ARP.Entries == nil {
		t.Fatal("empty ARP result must serialise as []")
	}

	job, _ = s.Job(context.Background(), "scan")
	if !job.PortScan.Cancelled || !job.PortScan.Truncated || !job.PortScan.Hosts[0].AlreadyRegistered || job.PortScan.Hosts[1].AlreadyRegistered {
		t.Fatalf("scan = %+v", job.PortScan)
	}
	if job, _ := s.Job(context.Background(), "scan-empty"); job.PortScan.Hosts == nil {
		t.Fatal("empty scan result must serialise as []")
	}
	if job, _ := s.Job(context.Background(), "snmp"); job.SNMP.SysName != "nas" {
		t.Fatalf("snmp = %+v", job.SNMP)
	}
	if job, _ := s.Job(context.Background(), "failed"); job.Error != "SNMP request failed: timeout" {
		t.Fatalf("error = %q", job.Error)
	}
	for _, id := range []string{"garbage", "weird"} {
		if job, _ := s.Job(context.Background(), id); job.Status != "failed" || job.Error == "" {
			t.Fatalf("%s: %+v", id, job)
		}
	}
	if job, _ := s.Job(context.Background(), "running"); job.Status != "running" || job.PortScan != nil {
		t.Fatalf("running = %+v", job)
	}
	if _, err := s.Job(context.Background(), "missing"); err == nil {
		t.Fatal("missing job found")
	}

	// Host lookup failure only drops the cross-reference.
	repo.allErr = errors.New("db down")
	if job, _ := s.Job(context.Background(), "scan"); job.PortScan.Hosts[0].AlreadyRegistered {
		t.Fatal("cross-reference without hosts")
	}
}

func TestJobErrorTruncated(t *testing.T) {
	if got := jobError("ERROR: " + strings.Repeat("é", 400)); len(got) > maxJobErrorLen {
		t.Fatalf("len = %d", len(got))
	}
}

func TestVendorHintAndLocalBit(t *testing.T) {
	cases := map[string]string{"00:50:56:aa:bb:cc": "VMware", "B8:27:EB:00:00:01": "Raspberry Pi", "aa:bb:cc:00:00:00": "", "short": ""}
	for mac, want := range cases {
		if got := VendorHint(mac); got != want {
			t.Errorf("%s: %q", mac, got)
		}
	}
	if locallyAdministered("") || locallyAdministered("zz:00") || locallyAdministered("00:11") || !locallyAdministered("02:00") {
		t.Fatal("U/L bit")
	}
}
