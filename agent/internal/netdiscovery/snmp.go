package netdiscovery

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gosnmp/gosnmp"
)

// SNMP system-group OIDs (RFC 1213 / SNMPv2-MIB).
const (
	oidSysDescr    = "1.3.6.1.2.1.1.1.0"
	oidSysObjectID = "1.3.6.1.2.1.1.2.0"
	oidSysUpTime   = "1.3.6.1.2.1.1.3.0"
	oidSysContact  = "1.3.6.1.2.1.1.4.0"
	oidSysName     = "1.3.6.1.2.1.1.5.0"
	oidSysLocation = "1.3.6.1.2.1.1.6.0"

	// ifTable columns (IF-MIB) walked for the interface summary.
	oidIfDescr       = "1.3.6.1.2.1.2.2.1.2"
	oidIfSpeed       = "1.3.6.1.2.1.2.2.1.5"
	oidIfPhysAddress = "1.3.6.1.2.1.2.2.1.6"
	oidIfAdminStatus = "1.3.6.1.2.1.2.2.1.7"
	oidIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"
)

const (
	// MaxSNMPInterfaces caps the interface summary (a core switch can expose
	// hundreds of ifTable rows).
	MaxSNMPInterfaces = 64
	// maxSNMPString truncates any string a device returns.
	maxSNMPString = 512
	// snmpOverallTimeout bounds the whole probe (GET + optional walks).
	snmpOverallTimeout = 45 * time.Second
	defaultSNMPTimeout = 2 * time.Second
	maxSNMPTimeout     = 10 * time.Second
)

// SNMPSecrets carries the credentials. The server never stores them: it
// injects them into the payload only when the agent claims the command, and
// they are never logged nor echoed in the result.
type SNMPSecrets struct {
	Community      string `json:"community,omitempty"`
	AuthPassphrase string `json:"auth_passphrase,omitempty"`
	PrivPassphrase string `json:"priv_passphrase,omitempty"`
}

// SNMPRequest is the discovery/snmp command payload.
type SNMPRequest struct {
	Target            string      `json:"target"`
	Port              int         `json:"port,omitempty"`
	Version           string      `json:"version"` // "2c" | "3"
	TimeoutMs         int         `json:"timeout_ms,omitempty"`
	IncludeInterfaces bool        `json:"include_interfaces,omitempty"`
	Username          string      `json:"username,omitempty"`
	SecurityLevel     string      `json:"security_level,omitempty"` // noAuthNoPriv | authNoPriv | authPriv
	AuthProtocol      string      `json:"auth_protocol,omitempty"`  // SHA | SHA256
	PrivProtocol      string      `json:"priv_protocol,omitempty"`  // AES
	Secrets           SNMPSecrets `json:"secrets"`
}

// SNMPInterface is one ifTable row summary.
type SNMPInterface struct {
	Index       int    `json:"index"`
	Descr       string `json:"descr"`
	SpeedBps    uint64 `json:"speed_bps"`
	PhysAddress string `json:"phys_address,omitempty"`
	AdminStatus string `json:"admin_status"`
	OperStatus  string `json:"oper_status"`
}

// SNMPResult is the discovery/snmp command output.
type SNMPResult struct {
	Target              string          `json:"target"`
	Version             string          `json:"version"`
	SysDescr            string          `json:"sys_descr"`
	SysObjectID         string          `json:"sys_object_id"`
	SysUpTimeSeconds    uint64          `json:"sys_uptime_seconds"`
	SysContact          string          `json:"sys_contact"`
	SysName             string          `json:"sys_name"`
	SysLocation         string          `json:"sys_location"`
	Interfaces          []SNMPInterface `json:"interfaces,omitempty"`
	InterfacesTruncated bool            `json:"interfaces_truncated,omitempty"`
	InterfacesError     string          `json:"interfaces_error,omitempty"`
}

// SNMPSession is the subset of gosnmp used here, so tests can mock a device.
type SNMPSession interface {
	Connect() error
	Get(oids []string) (*gosnmp.SnmpPacket, error)
	BulkWalkAll(rootOid string) ([]gosnmp.SnmpPDU, error)
	Close() error
}

// SessionFactory builds a session from a validated client configuration.
type SessionFactory func(g *gosnmp.GoSNMP) SNMPSession

type gosnmpSession struct{ *gosnmp.GoSNMP }

func (s gosnmpSession) Close() error {
	if s.Conn == nil {
		return nil
	}
	return s.Conn.Close()
}

// DefaultSessionFactory wraps a real gosnmp client.
func DefaultSessionFactory(g *gosnmp.GoSNMP) SNMPSession { return gosnmpSession{g} }

// BuildClient validates req and returns the gosnmp configuration. Errors
// never include a credential value.
func BuildClient(ctx context.Context, req SNMPRequest) (*gosnmp.GoSNMP, error) {
	addr, err := ParseTarget(req.Target)
	if err != nil {
		return nil, err
	}
	port := req.Port
	if port == 0 {
		port = 161
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid SNMP port %d", req.Port)
	}
	g := &gosnmp.GoSNMP{
		Target:             addr.String(),
		Port:               uint16(port),
		Timeout:            clampDuration(time.Duration(req.TimeoutMs)*time.Millisecond, defaultSNMPTimeout, 500*time.Millisecond, maxSNMPTimeout),
		Retries:            1,
		Context:            ctx,
		MaxOids:            gosnmp.MaxOids,
		MaxRepetitions:     32,
		ExponentialTimeout: false,
	}
	switch req.Version {
	case "2c":
		if req.Secrets.Community == "" {
			return nil, errors.New("SNMP community is missing (credentials expire if the command waits too long)")
		}
		g.Version = gosnmp.Version2c
		g.Community = req.Secrets.Community
	case "3":
		if err := configureV3(g, req); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported SNMP version %q", req.Version)
	}
	return g, nil
}

func configureV3(g *gosnmp.GoSNMP, req SNMPRequest) error {
	if req.Username == "" {
		return errors.New("SNMPv3 username is missing")
	}
	usm := &gosnmp.UsmSecurityParameters{UserName: req.Username}
	g.Version = gosnmp.Version3
	g.SecurityModel = gosnmp.UserSecurityModel
	switch req.SecurityLevel {
	case "noAuthNoPriv":
		g.MsgFlags = gosnmp.NoAuthNoPriv
	case "authNoPriv", "authPriv":
		proto, err := authProtocol(req.AuthProtocol)
		if err != nil {
			return err
		}
		if req.Secrets.AuthPassphrase == "" {
			return errors.New("SNMPv3 authentication passphrase is missing (credentials expire if the command waits too long)")
		}
		usm.AuthenticationProtocol = proto
		usm.AuthenticationPassphrase = req.Secrets.AuthPassphrase
		usm.PrivacyProtocol = gosnmp.NoPriv
		g.MsgFlags = gosnmp.AuthNoPriv
		if req.SecurityLevel == "authPriv" {
			if req.PrivProtocol != "AES" {
				return fmt.Errorf("unsupported SNMPv3 privacy protocol %q", req.PrivProtocol)
			}
			if req.Secrets.PrivPassphrase == "" {
				return errors.New("SNMPv3 privacy passphrase is missing (credentials expire if the command waits too long)")
			}
			usm.PrivacyProtocol = gosnmp.AES
			usm.PrivacyPassphrase = req.Secrets.PrivPassphrase
			g.MsgFlags = gosnmp.AuthPriv
		}
	default:
		return fmt.Errorf("unsupported SNMPv3 security level %q", req.SecurityLevel)
	}
	g.SecurityParameters = usm
	return nil
}

func authProtocol(name string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch name {
	case "SHA":
		return gosnmp.SHA, nil
	case "SHA256":
		return gosnmp.SHA256, nil
	default:
		return gosnmp.NoAuth, fmt.Errorf("unsupported SNMPv3 authentication protocol %q", name)
	}
}

// QuerySNMP runs the system-group GET (and the optional ifTable summary)
// against req.Target.
func QuerySNMP(ctx context.Context, req SNMPRequest, factory SessionFactory) (*SNMPResult, error) {
	ctx, cancel := context.WithTimeout(ctx, snmpOverallTimeout)
	defer cancel()
	g, err := BuildClient(ctx, req)
	if err != nil {
		return nil, err
	}
	if factory == nil {
		factory = DefaultSessionFactory
	}
	sess := factory(g)
	if err := sess.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connection to %s failed: %w", g.Target, err)
	}
	defer func() { _ = sess.Close() }()

	pkt, err := sess.Get([]string{oidSysDescr, oidSysObjectID, oidSysUpTime, oidSysContact, oidSysName, oidSysLocation})
	if err != nil {
		return nil, fmt.Errorf("SNMP request to %s failed: %w", g.Target, err)
	}
	if pkt.Error != gosnmp.NoError {
		return nil, fmt.Errorf("SNMP agent %s returned an error: %s", g.Target, pkt.Error)
	}
	res := &SNMPResult{Target: g.Target, Version: req.Version}
	applySystemPDUs(res, pkt.Variables)

	if req.IncludeInterfaces {
		ifaces, truncated, err := walkInterfaces(sess)
		if err != nil {
			res.InterfacesError = err.Error()
		} else {
			res.Interfaces = ifaces
			res.InterfacesTruncated = truncated
		}
	}
	return res, nil
}

func applySystemPDUs(res *SNMPResult, pdus []gosnmp.SnmpPDU) {
	for _, pdu := range pdus {
		switch strings.TrimPrefix(pdu.Name, ".") {
		case oidSysDescr:
			res.SysDescr = pduString(pdu)
		case oidSysObjectID:
			res.SysObjectID = strings.TrimPrefix(pduString(pdu), ".")
		case oidSysUpTime:
			res.SysUpTimeSeconds = pduUint(pdu) / 100 // TimeTicks are hundredths of a second
		case oidSysContact:
			res.SysContact = pduString(pdu)
		case oidSysName:
			res.SysName = pduString(pdu)
		case oidSysLocation:
			res.SysLocation = pduString(pdu)
		}
	}
}

func walkInterfaces(sess SNMPSession) ([]SNMPInterface, bool, error) {
	byIndex := map[int]*SNMPInterface{}
	var order []int
	get := func(idx int) *SNMPInterface {
		if it, ok := byIndex[idx]; ok {
			return it
		}
		it := &SNMPInterface{Index: idx}
		byIndex[idx] = it
		order = append(order, idx)
		return it
	}
	columns := []struct {
		oid   string
		apply func(*SNMPInterface, gosnmp.SnmpPDU)
	}{
		{oidIfDescr, func(it *SNMPInterface, p gosnmp.SnmpPDU) { it.Descr = pduString(p) }},
		{oidIfSpeed, func(it *SNMPInterface, p gosnmp.SnmpPDU) { it.SpeedBps = pduUint(p) }},
		{oidIfPhysAddress, func(it *SNMPInterface, p gosnmp.SnmpPDU) { it.PhysAddress = pduMAC(p) }},
		{oidIfAdminStatus, func(it *SNMPInterface, p gosnmp.SnmpPDU) { it.AdminStatus = ifStatus(pduUint(p)) }},
		{oidIfOperStatus, func(it *SNMPInterface, p gosnmp.SnmpPDU) { it.OperStatus = ifStatus(pduUint(p)) }},
	}
	for _, col := range columns {
		pdus, err := sess.BulkWalkAll(col.oid)
		if err != nil {
			return nil, false, fmt.Errorf("ifTable walk failed: %w", err)
		}
		for _, p := range pdus {
			idx, ok := ifIndex(p.Name, col.oid)
			if !ok {
				continue
			}
			col.apply(get(idx), p)
		}
	}
	truncated := len(order) > MaxSNMPInterfaces
	if truncated {
		order = order[:MaxSNMPInterfaces]
	}
	out := make([]SNMPInterface, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	return out, truncated, nil
}

func ifIndex(name, column string) (int, bool) {
	rest, ok := strings.CutPrefix(strings.TrimPrefix(name, "."), column+".")
	if !ok {
		return 0, false
	}
	idx, err := strconv.Atoi(rest)
	return idx, err == nil
}

func ifStatus(v uint64) string {
	switch v {
	case 1:
		return "up"
	case 2:
		return "down"
	case 3:
		return "testing"
	case 5:
		return "dormant"
	case 6:
		return "notPresent"
	case 7:
		return "lowerLayerDown"
	default:
		return "unknown"
	}
}

func pduString(p gosnmp.SnmpPDU) string {
	var s string
	switch v := p.Value.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return ""
	}
	return sanitize(s)
}

func pduUint(p gosnmp.SnmpPDU) uint64 {
	switch p.Type {
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Counter64, gosnmp.Uinteger32:
		b := gosnmp.ToBigInt(p.Value)
		if b.Sign() < 0 || !b.IsUint64() {
			return 0
		}
		return b.Uint64()
	default:
		return 0
	}
}

func pduMAC(p gosnmp.SnmpPDU) string {
	b, ok := p.Value.([]byte)
	if !ok || len(b) != 6 {
		return ""
	}
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}
	return strings.Join(parts, ":")
}

// sanitize drops control characters and truncates device-provided text.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if len(s) > maxSNMPString {
		s = strings.ToValidUTF8(s[:maxSNMPString], "")
	}
	return s
}
