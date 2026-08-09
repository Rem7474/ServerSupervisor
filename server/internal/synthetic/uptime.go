// Package synthetic implements server-side synthetic monitoring: uptime probes
// (HTTP / TCP) and SSL/TLS certificate expiration checks. Both run as background
// goroutines started from main.go and write results back to the database.
package synthetic

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// UptimeDB is the subset of database.DB methods needed by the uptime worker.
type UptimeDB interface {
	ListEnabledUptimeProbesDue(ctx context.Context) ([]models.UptimeProbe, error)
	RecordUptimeProbeResult(ctx context.Context, r models.UptimeProbeResult) error
	CleanupOldUptimeResults(ctx context.Context, olderThan time.Duration) (int64, error)
}

const (
	// uptimeTick is how often the worker wakes up to look for due probes.
	uptimeTick = 10 * time.Second
	// resultRetention is how long we keep individual probe result rows.
	resultRetention = 30 * 24 * time.Hour
)

// RunUptimeWorker runs the uptime probe loop until ctx is cancelled.
// Wire from main.go via background.Job, or call directly in a goroutine.
func RunUptimeWorker(ctx context.Context, db UptimeDB) {
	tick := time.NewTicker(uptimeTick)
	defer tick.Stop()
	cleanup := time.NewTicker(6 * time.Hour)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			runDueProbes(ctx, db)
		case <-cleanup.C:
			if n, err := db.CleanupOldUptimeResults(ctx, resultRetention); err == nil && n > 0 {
				// Best-effort log via stdlib log in caller; keep this package quiet.
				_ = n
			}
		}
	}
}

func runDueProbes(ctx context.Context, db UptimeDB) {
	probes, err := db.ListEnabledUptimeProbesDue(ctx)
	if err != nil {
		slog.WarnContext(ctx, "uptime: failed to list due probes", slog.Any("err", err))
		return
	}
	var wg sync.WaitGroup
	// Cap concurrency so a burst of probes can't fork hundreds of goroutines.
	sem := make(chan struct{}, 16)
	for _, p := range probes {
		wg.Add(1)
		sem <- struct{}{}
		go func(probe models.UptimeProbe) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if rec := recover(); rec != nil {
					slog.ErrorContext(ctx, "uptime: probe panicked",
						slog.String("probe_id", probe.ID),
						slog.Any("panic", rec),
						slog.String("stack", string(debug.Stack())))
				}
			}()
			result := executeProbe(ctx, probe)
			if err := db.RecordUptimeProbeResult(ctx, result); err != nil {
				slog.WarnContext(ctx, "uptime: failed to record probe result",
					slog.String("probe_id", probe.ID), slog.Any("err", err))
			}
		}(p)
	}
	wg.Wait()
}

// RunOnce performs the synthetic check for one probe and returns the result.
// Used by the on-demand "check now" handler.
func RunOnce(ctx context.Context, p models.UptimeProbe) models.UptimeProbeResult {
	return executeProbe(ctx, p)
}

// executeProbe performs the synthetic check for one probe and returns the result.
// Always returns a usable result — failures are encoded in Success=false + Error.
func executeProbe(ctx context.Context, p models.UptimeProbe) models.UptimeProbeResult {
	timeout := time.Duration(p.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch strings.ToLower(p.Type) {
	case "tcp":
		return checkTCP(checkCtx, p)
	case "icmp":
		return checkICMP(checkCtx, p)
	default: // "http"
		return checkHTTP(checkCtx, p, timeout)
	}
}

func checkTCP(ctx context.Context, p models.UptimeProbe) models.UptimeProbeResult {
	result := models.UptimeProbeResult{
		ProbeID:   p.ID,
		CheckedAt: time.Now(),
	}
	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", p.Target)
	result.LatencyMs = int(time.Since(start) / time.Millisecond)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	result.Success = true
	return result
}

// icmpProtocolICMPv4 is IANA's protocol number for ICMPv4 (1) — the value
// icmp.ParseMessage needs to know how to interpret the reply bytes. Not
// exported by golang.org/x/net/icmp (it lives in an internal subpackage),
// so it's a well-known literal here, same as the package's own examples.
const icmpProtocolICMPv4 = 1

// listenICMP opens an ICMPv4 echo socket, preferring the unprivileged "ping
// socket" (SOCK_DGRAM, needs no capability but requires the host/container's
// net.ipv4.ping_group_range sysctl to include this process's group — not
// configured by default) and falling back to a raw socket (needs
// CAP_NET_RAW — see server/Dockerfile's setcap step, which grants exactly
// that to the non-root server binary). The returned network name tells the
// caller which net.Addr type WriteTo needs.
func listenICMP() (conn *icmp.PacketConn, network string, err error) {
	if conn, err = icmp.ListenPacket("udp4", "0.0.0.0"); err == nil {
		return conn, "udp4", nil
	}
	conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, "", fmt.Errorf("ICMP indisponible (CAP_NET_RAW manquant sur le conteneur ? voir server/Dockerfile) : %w", err)
	}
	return conn, "ip4:icmp", nil
}

// checkICMP sends a single ICMPv4 echo request and waits for the matching
// reply — a generic "is this IP alive" check for equipment that isn't
// agent-installable and exposes no TCP/HTTP port to probe instead (switches,
// printers, IP cameras, ...). IPv6 targets aren't supported in this MVP.
func checkICMP(ctx context.Context, p models.UptimeProbe) models.UptimeProbeResult {
	result := models.UptimeProbeResult{ProbeID: p.ID, CheckedAt: time.Now()}
	success, latencyMs, err := PingICMP(ctx, p.Target)
	result.Success = success
	result.LatencyMs = latencyMs
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// PingICMP sends a single ICMPv4 echo request to target (a hostname or IPv4
// literal) and waits for the matching reply, bounded by ctx. Exported so
// other packages needing a generic "is this IP alive" check — e.g. the
// subnet discovery scan (internal/services/discovery) — can reuse the same
// socket-handling and CAP_NET_RAW fallback logic as the uptime ICMP probe
// instead of duplicating it.
func PingICMP(ctx context.Context, target string) (success bool, latencyMs int, err error) {
	dst, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return false, 0, fmt.Errorf("résolution de %q impossible : %w", target, err)
	}

	conn, network, err := listenICMP()
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("serversupervisor-icmp-probe"),
		},
	}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return false, 0, fmt.Errorf("échec construction paquet ICMP : %w", err)
	}

	var dstAddr net.Addr = &net.IPAddr{IP: dst.IP}
	if network == "udp4" {
		dstAddr = &net.UDPAddr{IP: dst.IP}
	}

	start := time.Now()
	if _, err := conn.WriteTo(wb, dstAddr); err != nil {
		return false, 0, fmt.Errorf("envoi ICMP échoué : %w", err)
	}

	rb := make([]byte, 1500)
	n, _, err := conn.ReadFrom(rb)
	latencyMs = int(time.Since(start) / time.Millisecond)
	if err != nil {
		return false, latencyMs, fmt.Errorf("aucune réponse ICMP : %w", err)
	}

	rm, err := icmp.ParseMessage(icmpProtocolICMPv4, rb[:n])
	if err != nil {
		return false, latencyMs, fmt.Errorf("réponse ICMP illisible : %w", err)
	}
	if rm.Type != ipv4.ICMPTypeEchoReply {
		return false, latencyMs, fmt.Errorf("type ICMP inattendu : %v", rm.Type)
	}

	return true, latencyMs, nil
}

func checkHTTP(ctx context.Context, p models.UptimeProbe, timeout time.Duration) models.UptimeProbeResult {
	result := models.UptimeProbeResult{
		ProbeID:   p.ID,
		CheckedAt: time.Now(),
	}

	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: !p.VerifyTLS}, //nolint:gosec
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if !p.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.Target, nil)
	if err != nil {
		result.Error = fmt.Sprintf("bad target: %v", err)
		return result
	}
	req.Header.Set("User-Agent", "ServerSupervisor-Uptime/1.0")

	start := time.Now()
	resp, err := client.Do(req)
	result.LatencyMs = int(time.Since(start) / time.Millisecond)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	status := resp.StatusCode
	result.StatusCode = &status

	if p.ExpectedStatus > 0 && resp.StatusCode != p.ExpectedStatus {
		result.Error = fmt.Sprintf("unexpected status %d (want %d)", resp.StatusCode, p.ExpectedStatus)
		return result
	}

	if p.ExpectedBodyRegex != "" {
		// Cap body reading to 256 KiB to avoid eating memory on huge responses.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		re, err := regexp.Compile(p.ExpectedBodyRegex)
		if err != nil {
			result.Error = fmt.Sprintf("bad expected_body_regex: %v", err)
			return result
		}
		if !re.Match(body) {
			result.Error = "body did not match expected_body_regex"
			return result
		}
	}

	result.Success = true
	return result
}

// Ensure database.DB satisfies UptimeDB at compile time when wired from main.go.
var _ UptimeDB = (*database.DB)(nil)
