// Package synthetic implements server-side synthetic monitoring: uptime probes
// (HTTP / TCP) and SSL/TLS certificate expiration checks. Both run as background
// goroutines started from main.go and write results back to the database.
package synthetic

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

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
			defer func() { recover() }() //nolint:errcheck
			result := executeProbe(ctx, probe)
			_ = db.RecordUptimeProbeResult(ctx, result)
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
