package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/agent/internal/netdiscovery"
	"github.com/serversupervisor/agent/internal/sender"
)

// Seams for tests: the ARP reader, the TCP dialer and the SNMP session
// factory. Production code uses the real kernel table / network.
var (
	readARPTable                                 = netdiscovery.ReadARPTable
	scanDialer       netdiscovery.DialFunc       = nil
	snmpFactory      netdiscovery.SessionFactory = nil
	progressReporter                             = func(ctx context.Context, s *sender.Sender, commandID, chunk string) bool {
		return s.StreamProgress(ctx, commandID, chunk)
	}
)

// handleDiscovery runs the network discovery module: arp (passive read of
// the kernel neighbour table), port_scan and snmp (active probes, only when
// the host's own config opts in with allow_network_scan). The payload is
// never logged: an SNMP payload carries credentials.
func handleDiscovery(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	reportRunning(ctx, s, cmd)

	var (
		out any
		err error
	)
	switch cmd.Action {
	case "arp":
		out, err = readARPTable()
	case "port_scan":
		out, err = runPortScan(ctx, d, s, cmd)
	case "snmp":
		out, err = runSNMP(ctx, d, cmd)
	default:
		err = fmt.Errorf("unknown discovery action: %s", cmd.Action)
	}

	if err != nil {
		slog.Warn("discovery command failed", "command_id", cmd.ID, "action", cmd.Action, "err", err)
		reportTerminal(ctx, s, cmd, "failed", "ERROR: "+err.Error())
		return
	}
	data, err := json.Marshal(out)
	if err != nil {
		reportTerminal(ctx, s, cmd, "failed", "ERROR: cannot encode the result")
		return
	}
	reportTerminal(ctx, s, cmd, "completed", string(data))
}

func scanAllowed(d *Dispatcher) bool {
	return d != nil && d.cfg != nil && d.cfg.AllowNetworkScan
}

func runPortScan(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) (*netdiscovery.ScanResult, error) {
	if !scanAllowed(d) {
		return nil, netdiscovery.ErrScanDisabled
	}
	var req netdiscovery.ScanRequest
	if err := json.Unmarshal([]byte(cmd.Payload), &req); err != nil {
		return nil, errors.New("invalid port scan payload")
	}
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	progress := func(p netdiscovery.Progress) {
		chunk, _ := json.Marshal(p)
		if progressReporter(ctx, s, cmd.ID, string(chunk)) {
			slog.Info("port scan cancelled by the server", "command_id", cmd.ID)
			cancel()
		}
	}
	return netdiscovery.Scan(scanCtx, req, scanDialer, progress)
}

func runSNMP(ctx context.Context, d *Dispatcher, cmd sender.PendingCommand) (*netdiscovery.SNMPResult, error) {
	if !scanAllowed(d) {
		return nil, netdiscovery.ErrScanDisabled
	}
	var req netdiscovery.SNMPRequest
	if err := json.Unmarshal([]byte(cmd.Payload), &req); err != nil {
		return nil, errors.New("invalid SNMP payload")
	}
	return netdiscovery.QuerySNMP(ctx, req, snmpFactory)
}
