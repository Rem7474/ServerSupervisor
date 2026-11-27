package netdiscovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// fakeSession is a mocked SNMP device.
type fakeSession struct {
	cfg        *gosnmp.GoSNMP
	connectErr error
	getErr     error
	getPkt     *gosnmp.SnmpPacket
	walks      map[string][]gosnmp.SnmpPDU
	walkErr    error
	closed     bool
	getOIDs    []string
}

func (f *fakeSession) Connect() error { return f.connectErr }
func (f *fakeSession) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	f.getOIDs = oids
	return f.getPkt, f.getErr
}
func (f *fakeSession) BulkWalkAll(root string) ([]gosnmp.SnmpPDU, error) {
	if f.walkErr != nil {
		return nil, f.walkErr
	}
	return f.walks[root], nil
}
func (f *fakeSession) Close() error { f.closed = true; return nil }

func (f *fakeSession) factory(g *gosnmp.GoSNMP) SNMPSession {
	f.cfg = g
	return f
}

const testCommunity = "test-community-value"

func systemPacket() *gosnmp.SnmpPacket {
	return &gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{
		{Name: "." + oidSysDescr, Type: gosnmp.OctetString, Value: []byte("Linux switch\x00 5.10\n")},
		{Name: "." + oidSysObjectID, Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.4.1.8072.3.2.10"},
		{Name: "." + oidSysUpTime, Type: gosnmp.TimeTicks, Value: uint32(123456)},
		{Name: "." + oidSysContact, Type: gosnmp.OctetString, Value: []byte("noc@example.invalid")},
		{Name: "." + oidSysName, Type: gosnmp.OctetString, Value: []byte("core-sw")},
		{Name: "." + oidSysLocation, Type: gosnmp.OctetString, Value: []byte("Rack 1")},
	}}
}

func v2cRequest() SNMPRequest {
	return SNMPRequest{Target: "192.168.1.2", Version: "2c", Secrets: SNMPSecrets{Community: testCommunity}}
}

func TestQuerySNMPv2cSystemGroup(t *testing.T) {
	fs := &fakeSession{getPkt: systemPacket()}
	res, err := QuerySNMP(context.Background(), v2cRequest(), fs.factory)
	if err != nil {
		t.Fatal(err)
	}
	if fs.cfg.Version != gosnmp.Version2c || fs.cfg.Community != testCommunity || fs.cfg.Port != 161 || fs.cfg.Target != "192.168.1.2" {
		t.Fatalf("client config = %+v", fs.cfg)
	}
	if len(fs.getOIDs) != 6 {
		t.Fatalf("GET asked %d OIDs", len(fs.getOIDs))
	}
	want := SNMPResult{
		Target: "192.168.1.2", Version: "2c",
		SysDescr: "Linux switch 5.10", SysObjectID: "1.3.6.1.4.1.8072.3.2.10",
		SysUpTimeSeconds: 1234, SysContact: "noc@example.invalid", SysName: "core-sw", SysLocation: "Rack 1",
	}
	if fmt.Sprintf("%+v", *res) != fmt.Sprintf("%+v", want) {
		t.Fatalf("result = %+v\nwant     %+v", *res, want)
	}
	if !fs.closed {
		t.Error("session must be closed")
	}
}

func TestQuerySNMPInterfaces(t *testing.T) {
	fs := &fakeSession{getPkt: systemPacket(), walks: map[string][]gosnmp.SnmpPDU{
		oidIfDescr: {
			{Name: "." + oidIfDescr + ".1", Type: gosnmp.OctetString, Value: []byte("lo")},
			{Name: "." + oidIfDescr + ".2", Type: gosnmp.OctetString, Value: []byte("eth0")},
			{Name: ".1.3.6.1.2.1.2.2.1.2.x", Type: gosnmp.OctetString, Value: []byte("bad index")},
		},
		oidIfSpeed:       {{Name: "." + oidIfSpeed + ".2", Type: gosnmp.Gauge32, Value: uint(1000000000)}},
		oidIfPhysAddress: {{Name: "." + oidIfPhysAddress + ".2", Type: gosnmp.OctetString, Value: []byte{0xbc, 0x24, 0x11, 0, 1, 2}}, {Name: "." + oidIfPhysAddress + ".1", Type: gosnmp.OctetString, Value: []byte{}}},
		oidIfAdminStatus: {{Name: "." + oidIfAdminStatus + ".1", Type: gosnmp.Integer, Value: 1}, {Name: "." + oidIfAdminStatus + ".2", Type: gosnmp.Integer, Value: 2}},
		oidIfOperStatus:  {{Name: "." + oidIfOperStatus + ".1", Type: gosnmp.Integer, Value: 7}, {Name: "." + oidIfOperStatus + ".2", Type: gosnmp.Integer, Value: 42}},
	}}
	req := v2cRequest()
	req.IncludeInterfaces = true
	res, err := QuerySNMP(context.Background(), req, fs.factory)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Interfaces) != 2 || res.InterfacesTruncated {
		t.Fatalf("interfaces = %+v", res.Interfaces)
	}
	lo, eth := res.Interfaces[0], res.Interfaces[1]
	if lo.Index != 1 || lo.Descr != "lo" || lo.AdminStatus != "up" || lo.OperStatus != "lowerLayerDown" || lo.PhysAddress != "" {
		t.Errorf("lo = %+v", lo)
	}
	if eth.Index != 2 || eth.SpeedBps != 1000000000 || eth.PhysAddress != "bc:24:11:00:01:02" || eth.AdminStatus != "down" || eth.OperStatus != "unknown" {
		t.Errorf("eth0 = %+v", eth)
	}
}

func TestQuerySNMPInterfacesTruncatedAndErrors(t *testing.T) {
	var descr []gosnmp.SnmpPDU
	for i := 1; i <= MaxSNMPInterfaces+3; i++ {
		descr = append(descr, gosnmp.SnmpPDU{Name: fmt.Sprintf(".%s.%d", oidIfDescr, i), Type: gosnmp.OctetString, Value: []byte("p")})
	}
	fs := &fakeSession{getPkt: systemPacket(), walks: map[string][]gosnmp.SnmpPDU{oidIfDescr: descr}}
	req := v2cRequest()
	req.IncludeInterfaces = true
	res, err := QuerySNMP(context.Background(), req, fs.factory)
	if err != nil {
		t.Fatal(err)
	}
	if !res.InterfacesTruncated || len(res.Interfaces) != MaxSNMPInterfaces {
		t.Fatalf("truncated=%v len=%d", res.InterfacesTruncated, len(res.Interfaces))
	}

	// A failing walk keeps the system group and reports the interface error.
	fs = &fakeSession{getPkt: systemPacket(), walkErr: errors.New("request timeout")}
	res, err = QuerySNMP(context.Background(), req, fs.factory)
	if err != nil {
		t.Fatal(err)
	}
	if res.SysName != "core-sw" || !strings.Contains(res.InterfacesError, "request timeout") || res.Interfaces != nil {
		t.Fatalf("result = %+v", res)
	}
}

func TestQuerySNMPFailures(t *testing.T) {
	tests := []struct {
		name string
		fs   *fakeSession
		want string
	}{
		{"connect", &fakeSession{connectErr: errors.New("no route")}, "connection to 192.168.1.2 failed"},
		{"get", &fakeSession{getErr: errors.New("request timeout")}, "request to 192.168.1.2 failed"},
		{"agent error", &fakeSession{getPkt: &gosnmp.SnmpPacket{Error: gosnmp.AuthorizationError}}, "returned an error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := QuerySNMP(context.Background(), v2cRequest(), tt.fs.factory)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), testCommunity) {
				t.Fatal("the community must never appear in an error")
			}
		})
	}
}

func TestBuildClientValidation(t *testing.T) {
	tests := []struct {
		name string
		req  SNMPRequest
		want string
	}{
		{"public target", SNMPRequest{Target: "1.1.1.1", Version: "2c", Secrets: SNMPSecrets{Community: "x"}}, "not a private"},
		{"bad port", SNMPRequest{Target: "10.0.0.1", Port: 70000, Version: "2c", Secrets: SNMPSecrets{Community: "x"}}, "invalid SNMP port"},
		{"missing community", SNMPRequest{Target: "10.0.0.1", Version: "2c"}, "community is missing"},
		{"bad version", SNMPRequest{Target: "10.0.0.1", Version: "1"}, "unsupported SNMP version"},
		{"v3 no user", SNMPRequest{Target: "10.0.0.1", Version: "3", SecurityLevel: "noAuthNoPriv"}, "username is missing"},
		{"v3 bad level", SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "u", SecurityLevel: "x"}, "security level"},
		{"v3 bad auth proto", SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "u", SecurityLevel: "authNoPriv", AuthProtocol: "MD5", Secrets: SNMPSecrets{AuthPassphrase: "p"}}, "authentication protocol"},
		{"v3 missing auth pass", SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "u", SecurityLevel: "authNoPriv", AuthProtocol: "SHA"}, "authentication passphrase is missing"},
		{"v3 bad priv proto", SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "u", SecurityLevel: "authPriv", AuthProtocol: "SHA", PrivProtocol: "DES", Secrets: SNMPSecrets{AuthPassphrase: "p"}}, "privacy protocol"},
		{"v3 missing priv pass", SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "u", SecurityLevel: "authPriv", AuthProtocol: "SHA", PrivProtocol: "AES", Secrets: SNMPSecrets{AuthPassphrase: "p"}}, "privacy passphrase is missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildClient(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildClientV3Levels(t *testing.T) {
	g, err := BuildClient(context.Background(), SNMPRequest{Target: "10.0.0.1", Port: 1161, Version: "3", Username: "mon", SecurityLevel: "noAuthNoPriv", TimeoutMs: 99999})
	if err != nil {
		t.Fatal(err)
	}
	if g.MsgFlags != gosnmp.NoAuthNoPriv || g.Port != 1161 || g.Timeout != maxSNMPTimeout {
		t.Fatalf("noAuthNoPriv = %+v", g)
	}

	g, err = BuildClient(context.Background(), SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "mon", SecurityLevel: "authNoPriv", AuthProtocol: "SHA256", Secrets: SNMPSecrets{AuthPassphrase: "authpass1"}})
	if err != nil {
		t.Fatal(err)
	}
	usm := g.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if g.MsgFlags != gosnmp.AuthNoPriv || usm.AuthenticationProtocol != gosnmp.SHA256 || usm.PrivacyProtocol != gosnmp.NoPriv {
		t.Fatalf("authNoPriv = %+v %+v", g, usm)
	}

	g, err = BuildClient(context.Background(), SNMPRequest{Target: "10.0.0.1", Version: "3", Username: "mon", SecurityLevel: "authPriv", AuthProtocol: "SHA", PrivProtocol: "AES", Secrets: SNMPSecrets{AuthPassphrase: "authpass1", PrivPassphrase: "privpass1"}})
	if err != nil {
		t.Fatal(err)
	}
	usm = g.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if g.MsgFlags != gosnmp.AuthPriv || g.SecurityModel != gosnmp.UserSecurityModel || usm.AuthenticationProtocol != gosnmp.SHA || usm.PrivacyProtocol != gosnmp.AES || usm.UserName != "mon" {
		t.Fatalf("authPriv = %+v %+v", g, usm)
	}
}

func TestSanitizeAndConversions(t *testing.T) {
	long := strings.Repeat("é", maxSNMPString)
	if got := sanitize(long); len(got) > maxSNMPString || !strings.HasPrefix(got, "é") || strings.ContainsRune(got, '�') {
		t.Fatalf("sanitize did not truncate on a rune boundary: %d bytes", len(got))
	}
	if got := pduString(gosnmp.SnmpPDU{Value: 42}); got != "" {
		t.Errorf("non-string pdu = %q", got)
	}
	if got := pduUint(gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: -5}); got != 0 {
		t.Errorf("negative integer = %d", got)
	}
	if got := pduUint(gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte("1")}); got != 0 {
		t.Errorf("octet string as uint = %d", got)
	}
	if got := ifStatus(3) + "," + ifStatus(5) + "," + ifStatus(6); got != "testing,dormant,notPresent" {
		t.Errorf("ifStatus = %s", got)
	}
	if (gosnmpSession{&gosnmp.GoSNMP{}}).Close() != nil {
		t.Error("closing an unconnected session must be a no-op")
	}
	if DefaultSessionFactory(&gosnmp.GoSNMP{}) == nil {
		t.Error("default factory returned nil")
	}
}
