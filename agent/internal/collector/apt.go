package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	aptCommandIdleTimeout     = 10 * time.Minute
	aptIdleTimeoutCheckPeriod = 5 * time.Second
)

type CVEInfo struct {
	ID             string  `json:"id"`
	Severity       string  `json:"severity"`        // Mapped from UbuntuPriority
	UbuntuPriority string  `json:"ubuntu_priority"` // Raw Ubuntu priority (critical/high/medium/low/negligible)
	CVSSScore      float64 `json:"cvss_score"`      // CVSS v3 score (0 if unavailable)
	CVSSVector     string  `json:"cvss_vector,omitempty"`
	Package        string  `json:"package"`
}

type AptStatus struct {
	LastUpdate      time.Time `json:"last_update"`
	LastUpgrade     time.Time `json:"last_upgrade"`
	PendingPackages int       `json:"pending_packages"`
	PackageList     string    `json:"package_list"` // JSON array
	SecurityUpdates int       `json:"security_updates"`
	CVEList         string    `json:"cve_list"` // JSON array of CVEInfo
}

// CollectAPT checks for available APT updates
// If extractCVE is true, extracts CVE information (resource intensive)
func CollectAPT(extractCVE bool) (*AptStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status := &AptStatus{}

	// Check if apt-get is available
	if _, err := exec.LookPath("apt-get"); err != nil {
		return nil, fmt.Errorf("apt-get not found in PATH")
	}

	// Get last update time from apt history log
	status.LastUpdate = getLastAptAction("Start-Date", "/var/log/apt/history.log")
	status.LastUpgrade = getLastAptUpgrade()

	// List upgradable packages using dry-run simulation
	out, err := exec.CommandContext(ctx, "apt-get", "upgrade", "--simulate").Output()
	if err != nil {
		slog.Warn("apt-get upgrade --simulate failed", "err", err)
		return status, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var packages []string
	secCount := 0
	var cveInfos []CVEInfo

	for _, line := range lines {
		// Only process lines starting with "Inst " (package installation lines)
		if !strings.HasPrefix(line, "Inst ") {
			continue
		}

		// Format: "Inst package_name [current_version] (new_version ...)"
		// Extract package name (second token)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		packageName := fields[1]
		packages = append(packages, packageName)

		// Check if it's a security update by looking at policy
		if isSecurityUpdate(packageName) {
			secCount++
			// Extract CVEs for security packages only if requested
			if extractCVE {
				cves := extractCVEsForPackage(packageName)
				cveInfos = append(cveInfos, cves...)
			}
		}
	}

	status.PendingPackages = len(packages)
	status.SecurityUpdates = secCount

	// Build JSON array of package names
	if len(packages) > 0 {
		quoted := make([]string, len(packages))
		for i, p := range packages {
			quoted[i] = fmt.Sprintf("%q", p)
		}
		status.PackageList = "[" + strings.Join(quoted, ",") + "]"
	} else {
		status.PackageList = "[]"
	}

	// Build JSON array of CVE info
	if len(cveInfos) > 0 {
		cveJSON, err := json.Marshal(cveInfos)
		if err != nil {
			status.CVEList = "[]"
		} else {
			status.CVEList = string(cveJSON)
		}
	} else {
		status.CVEList = "[]"
	}

	if extractCVE {
		slog.Debug("apt status collected", "upgradable", status.PendingPackages, "security", status.SecurityUpdates, "cves", len(cveInfos))
	} else {
		slog.Debug("apt status collected", "upgradable", status.PendingPackages, "security", status.SecurityUpdates)
	}
	return status, nil
}

// ExecuteAptCommand runs an apt command (update, upgrade, dist-upgrade)
func ExecuteAptCommand(command string) (string, error) {
	return ExecuteAptCommandWithStreaming(command, nil)
}

func newAptGetCmd(command string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch command {
	case "update":
		cmd = exec.Command("apt-get", "update")
	case "upgrade":
		cmd = exec.Command("apt-get", "upgrade", "-y", "-q", "-o", "Dpkg::Options::=--force-confold")
	case "dist-upgrade":
		cmd = exec.Command("apt-get", "dist-upgrade", "-y", "-q", "-o", "Dpkg::Options::=--force-confold")
	default:
		return nil, fmt.Errorf("unknown apt command: %s", command)
	}
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	return cmd, nil
}

// ExecuteAptCommandWithStreaming runs an apt-get command with real-time output streaming
// streamCallback is called for each chunk of output (can be nil)
func ExecuteAptCommandWithStreaming(command string, streamCallback func(chunk string)) (string, error) {
	cmd, err := newAptGetCmd(command)
	if err != nil {
		return "", err
	}

	slog.Info("executing apt-get command", "command", command)

	output, runErr := runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)
	if runErr != nil && isDpkgInterruptedError(output, runErr) {
		slog.Warn("apt-get reported interrupted dpkg state, running dpkg --configure -a before retrying", "command", command)
		repairOutput, repairErr := runDpkgConfigureAll(streamCallback)
		if repairErr != nil {
			return output, fmt.Errorf("apt-get %s failed: %w\nOutput: %s\nRecovery dpkg --configure -a failed: %v\nRecovery output: %s", command, runErr, output, repairErr, repairOutput)
		}
		// Rebuild the command: *exec.Cmd cannot be reused after Start()/Wait()
		cmd, err = newAptGetCmd(command)
		if err != nil {
			return output, fmt.Errorf("apt-get %s retry setup failed: %w", command, err)
		}
		output, runErr = runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)
	}

	if runErr != nil {
		return output, fmt.Errorf("apt-get %s failed: %w\nOutput: %s", command, runErr, output)
	}

	slog.Info("apt-get command completed", "command", command)
	return output, nil
}

func isDpkgInterruptedError(output string, err error) bool {
	if err == nil {
		return false
	}
	combined := strings.ToLower(output + "\n" + err.Error())
	return strings.Contains(combined, "dpkg was interrupted") || strings.Contains(combined, "run 'dpkg --configure -a'")
}

func runDpkgConfigureAll(streamCallback func(chunk string)) (string, error) {
	cmd := exec.Command("dpkg", "--configure", "-a")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	slog.Info("executing dpkg --configure -a")
	return runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)
}

// runCommandWithStreaming executes a command and streams output via callback
func runCommandWithStreaming(cmd *exec.Cmd, streamCallback func(chunk string), idleTimeout time.Duration) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var fullOutput strings.Builder
	var mu sync.Mutex
	var lastChunkUnixNano atomic.Int64
	var timedOut atomic.Bool

	lastChunkUnixNano.Store(time.Now().UnixNano())

	handleChunk := func(chunk string) {
		lastChunkUnixNano.Store(time.Now().UnixNano())
		mu.Lock()
		fullOutput.WriteString(chunk)
		mu.Unlock()
		if streamCallback != nil {
			streamCallback(chunk)
		}
	}

	// Kill the process only when no output chunk has been produced for idleTimeout.
	monitorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(aptIdleTimeoutCheckPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-monitorDone:
				return
			case <-ticker.C:
				last := time.Unix(0, lastChunkUnixNano.Load())
				if time.Since(last) <= idleTimeout {
					continue
				}
				if timedOut.CompareAndSwap(false, true) {
					timeoutMsg := fmt.Sprintf("\nERROR: apt command timed out after %s without log output", idleTimeout)
					handleChunk(timeoutMsg)
					if cmd.Process != nil {
						_ = cmd.Process.Kill()
					}
				}
			}
		}
	}()

	// Read stdout and stderr concurrently - each goroutine has its own buffer.
	done := make(chan struct{}, 2)

	go func() {
		buf := make([]byte, 4096) // Local buffer for this goroutine
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				handleChunk(string(buf[:n]))
			}
			if err != nil {
				done <- struct{}{}
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096) // Local buffer for this goroutine
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				handleChunk(string(buf[:n]))
			}
			if err != nil {
				done <- struct{}{}
				return
			}
		}
	}()

	// Wait for both streams to finish
	<-done
	<-done
	close(monitorDone)

	err = cmd.Wait()
	if timedOut.Load() {
		return fullOutput.String(), fmt.Errorf("timeout after %s without log output", idleTimeout)
	}
	return fullOutput.String(), err
}

func getLastAptAction(prefix, logFile string) time.Time {
	data, err := os.ReadFile(logFile)
	if err != nil {
		return time.Time{}
	}
	var lastLine string
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			lastLine = line
		}
	}
	if lastLine == "" {
		return time.Time{}
	}
	parts := strings.SplitN(lastLine, ":", 2)
	if len(parts) != 2 {
		return time.Time{}
	}
	ts := strings.TrimSpace(parts[1])
	// APT history timestamps are in local time; preserve local timezone
	t, _ := time.ParseInLocation("2006-01-02  15:04:05", ts, time.Local)
	return t
}

func getLastAptUpgrade() time.Time {
	out, err := exec.Command("stat", "-c", "%Y", "/var/lib/dpkg/info").Output()
	if err != nil {
		return time.Time{}
	}
	ts := strings.TrimSpace(string(out))
	epoch, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}
	}
	// Preserve local timezone for UI relative time display
	return time.Unix(epoch, 0).In(time.Local)
}

// isSecurityUpdate checks if a package upgrade comes from security repositories
func isSecurityUpdate(packageName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "apt-cache", "policy", packageName).Output()
	if err != nil {
		return false
	}

	policy := string(out)
	// Check if any upgrade source contains "security"
	return strings.Contains(strings.ToLower(policy), "security.ubuntu.com") ||
		strings.Contains(strings.ToLower(policy), "-security")
}

// ─── Ubuntu CVE API ───────────────────────────────────────────────────────────

const (
	ubuntuCVEAPIBase = "https://ubuntu.com/security/cves/%s.json"
	cveCacheDir      = "/tmp/ss-cve-cache"
	cveCacheTTL      = 24 * time.Hour
	cveAPITimeout    = 20 * time.Second
	cveParallelLimit = 3 // keep low to avoid rate-limiting from ubuntu.com
)

// ubuntuCVEResponse is the subset of the Ubuntu CVE JSON API we care about.
// cvss3 can be null, a plain number, or an object — use RawMessage to handle all cases.
type ubuntuCVEResponse struct {
	ID       string          `json:"id"`
	Priority string          `json:"priority"` // critical / high / medium / low / negligible / unknown
	CVSS3Raw json.RawMessage `json:"cvss3"`
}

type ubuntuCVSS3 struct {
	Score  float64 `json:"score"`
	Vector string  `json:"vector"`
}

// cvss3 parses the raw cvss3 field, tolerating number, object, or null.
func (r *ubuntuCVEResponse) cvss3() *ubuntuCVSS3 {
	if len(r.CVSS3Raw) == 0 {
		return nil
	}
	// Try object form first
	var obj ubuntuCVSS3
	if err := json.Unmarshal(r.CVSS3Raw, &obj); err == nil && obj.Score > 0 {
		return &obj
	}
	// Try plain number (score only, no vector)
	var score float64
	if err := json.Unmarshal(r.CVSS3Raw, &score); err == nil && score > 0 {
		return &ubuntuCVSS3{Score: score}
	}
	return nil
}

// ubuntuPriorityToSeverity maps an Ubuntu priority label to an upper-cased severity string.
func ubuntuPriorityToSeverity(priority string) string {
	switch strings.ToLower(priority) {
	case "critical":
		return "CRITICAL"
	case "high":
		return "HIGH"
	case "medium":
		return "MEDIUM"
	case "low":
		return "LOW"
	case "negligible":
		return "NEGLIGIBLE"
	default:
		return "UNKNOWN"
	}
}

var cveHTTPClient = &http.Client{Timeout: cveAPITimeout}

// fetchUbuntuCVE fetches CVE data from Ubuntu's security API with one retry.
// Results are cached on disk for cveCacheTTL to avoid hammering the API.
func fetchUbuntuCVE(cveID string) (*ubuntuCVEResponse, error) {
	cacheFile := filepath.Join(cveCacheDir, cveID+".json")

	// Return cached result if still fresh.
	if info, err := os.Stat(cacheFile); err == nil && time.Since(info.ModTime()) < cveCacheTTL {
		data, err := os.ReadFile(cacheFile)
		if err == nil {
			var cached ubuntuCVEResponse
			if json.Unmarshal(data, &cached) == nil {
				return &cached, nil
			}
		}
	}

	url := fmt.Sprintf(ubuntuCVEAPIBase, cveID)

	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}

		result, body, err := doUbuntuCVERequest(url)
		if err == errCVENotFound {
			// Cache a sentinel so we don't retry for cveCacheTTL.
			sentinel, _ := json.Marshal(ubuntuCVEResponse{ID: cveID, Priority: "unknown"})
			_ = os.MkdirAll(cveCacheDir, 0o755)
			_ = os.WriteFile(cacheFile, sentinel, 0o644)
			return nil, err
		}
		if err != nil {
			lastErr = err
			continue
		}

		// Persist to cache (best-effort).
		_ = os.MkdirAll(cveCacheDir, 0o755)
		_ = os.WriteFile(cacheFile, body, 0o644)

		return result, nil
	}

	return nil, lastErr
}

// errCVENotFound is returned when the Ubuntu API has no data for a CVE ID.
// The caller caches a sentinel so we don't retry for cveCacheTTL.
var errCVENotFound = fmt.Errorf("CVE not found in Ubuntu database")

func doUbuntuCVERequest(url string) (*ubuntuCVEResponse, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cveAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ServerSupervisor-Agent/1.0")

	resp, err := cveHTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, errCVENotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("ubuntu CVE API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, nil, err
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil, errCVENotFound
	}

	var result ubuntuCVEResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, errCVENotFound
	}

	return &result, body, nil
}

// enrichCVEsWithUbuntuData queries the Ubuntu CVE API for each CVE ID (in parallel)
// and fills in UbuntuPriority, Severity, CVSSScore, and CVSSVector.
// Falls back to "UNKNOWN" if the API is unreachable.
func enrichCVEsWithUbuntuData(cves []CVEInfo) []CVEInfo {
	sem := make(chan struct{}, cveParallelLimit)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range cves {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := fetchUbuntuCVE(cves[idx].ID)
			if err != nil {
				slog.Debug("CVE API fetch failed", "cve", cves[idx].ID, "err", err)
				return
			}

			mu.Lock()
			cves[idx].UbuntuPriority = data.Priority
			cves[idx].Severity = ubuntuPriorityToSeverity(data.Priority)
			if cvss := data.cvss3(); cvss != nil {
				cves[idx].CVSSScore = cvss.Score
				cves[idx].CVSSVector = cvss.Vector
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	return cves
}

// ─── Changelog parsing ────────────────────────────────────────────────────────

// extractCVEsForPackage extracts CVE IDs from the package changelog (entries
// strictly newer than the installed version) then enriches them with official
// Ubuntu priority and CVSS data from the Ubuntu Security API.
func extractCVEsForPackage(packageName string) []CVEInfo {
	cveMap := make(map[string]bool)

	installedVersion := getInstalledPackageVersion(packageName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "apt-get", "changelog", packageName)
	output, err := cmd.Output()
	if err != nil {
		return extractCVEsFromPolicy(packageName)
	}

	changelog := extractChangelogSinceVersion(string(output), installedVersion, packageName)
	if changelog == "" {
		return nil
	}

	cveRegex := regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)
	var cves []CVEInfo
	for _, cveID := range cveRegex.FindAllString(changelog, -1) {
		if cveMap[cveID] {
			continue
		}
		cveMap[cveID] = true
		cves = append(cves, CVEInfo{
			ID:             cveID,
			Severity:       "UNKNOWN",
			UbuntuPriority: "unknown",
			Package:        packageName,
		})
	}

	if len(cves) > 0 {
		cves = enrichCVEsWithUbuntuData(cves)
	}

	return cves
}

// getInstalledPackageVersion returns the currently installed version of a package via dpkg-query.
func getInstalledPackageVersion(packageName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Version}", packageName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// debChangelogHeaderRe matches any standard Debian changelog entry header:
//
//	packagename (version) suite; urgency=level
//
// We intentionally ignore the package name so that source-package changelogs
// (e.g. "openssl" for the binary "libssl3") are parsed correctly.
var debChangelogHeaderRe = regexp.MustCompile(`(?m)^[\w.+-]+\s+\(([^)]+)\)\s+[^;]+;\s+urgency=`)

// extractChangelogSinceVersion returns only the changelog text for versions strictly
// newer than installedVersion. If installedVersion is empty, returns only the first
// (most recent) changelog entry to avoid scanning the entire history.
func extractChangelogSinceVersion(changelog, installedVersion, _ string) string {
	type entryBound struct {
		version string
		start   int
	}
	var bounds []entryBound
	for _, loc := range debChangelogHeaderRe.FindAllStringSubmatchIndex(changelog, -1) {
		version := changelog[loc[2]:loc[3]]
		bounds = append(bounds, entryBound{version: version, start: loc[0]})
	}

	if len(bounds) == 0 {
		// Unrecognised format — return nothing rather than risk surfacing old CVEs.
		return ""
	}

	if installedVersion == "" {
		// Unknown installed version: only scan the single most-recent entry.
		if len(bounds) > 1 {
			return changelog[bounds[0].start:bounds[1].start]
		}
		return changelog[bounds[0].start:]
	}

	// Walk entries (newest-first) and stop at the first one that is <= installedVersion.
	cutoff := len(changelog)
	for _, b := range bounds {
		if err := exec.Command("dpkg", "--compare-versions", b.version, "le", installedVersion).Run(); err == nil {
			cutoff = b.start
			break
		}
	}
	if cutoff == 0 {
		return ""
	}
	return changelog[:cutoff]
}

// extractCVEsFromPolicy is a fallback when the changelog is unavailable.
// It detects that a security update exists but cannot provide CVE-level detail.
func extractCVEsFromPolicy(packageName string) []CVEInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "apt-cache", "policy", packageName).Output()
	if err != nil {
		return nil
	}

	policy := string(out)
	cveRegex := regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)
	cveIDs := cveRegex.FindAllString(policy, -1)

	var cves []CVEInfo
	cveMap := make(map[string]bool)

	for _, id := range cveIDs {
		if cveMap[id] {
			continue
		}
		cveMap[id] = true
		cves = append(cves, CVEInfo{
			ID:             id,
			Severity:       "UNKNOWN",
			UbuntuPriority: "unknown",
			Package:        packageName,
		})
	}

	// Changelog unavailable but we know it's a security update.
	if len(cves) == 0 && (strings.Contains(policy, "security.ubuntu.com") || strings.Contains(policy, "-security")) {
		cves = append(cves, CVEInfo{
			ID:             "SECURITY-UPDATE",
			Severity:       "UNKNOWN",
			UbuntuPriority: "unknown",
			Package:        packageName,
		})
	}

	if len(cves) > 0 {
		cves = enrichCVEsWithUbuntuData(cves)
	}

	return cves
}

// ========== Unattended-Upgrades ==========

const uuLogPath = "/var/log/unattended-upgrades/unattended-upgrades.log"
const uuAutoUpgradesConf = "/etc/apt/apt.conf.d/20auto-upgrades"
const uuMainConf = "/etc/apt/apt.conf.d/50unattended-upgrades"
const uuRebootRequired = "/var/run/reboot-required"
const uuInitialBackfillBytes int64 = 256 * 1024
const uuInitialMaxRuns = 20

// uuLogCursor tracks the byte offset in the UU log file.
// -1 means "not yet initialised": on the first call we seek to EOF so we
// don't flood the server with historical upgrade runs from before the agent started.
var uuLogCursor int64 = -1
var uuLogMu sync.Mutex

type UUConfig struct {
	SecurityOnly   bool   `json:"security_only"`
	AutoReboot     bool   `json:"auto_reboot"`
	AutoRebootTime string `json:"auto_reboot_time"`
	RemoveUnused   bool   `json:"remove_unused"`
}

type UURun struct {
	RunAt      time.Time `json:"run_at"`
	Packages   []string  `json:"packages"`
	HadError   bool      `json:"had_error"`
	LogSnippet string    `json:"log_snippet,omitempty"`
}

type UnattendedUpgradesStatus struct {
	Installed      bool     `json:"installed"`
	Enabled        bool     `json:"enabled"`
	RebootRequired bool     `json:"reboot_required"`
	Config         UUConfig `json:"config"`
	NewRuns        []UURun  `json:"new_runs"`
}

// CollectUnattendedUpgrades collects the current status of unattended-upgrades.
// NewRuns contains only upgrade runs discovered since the previous call (cursor-based).
func CollectUnattendedUpgrades() *UnattendedUpgradesStatus {
	status := &UnattendedUpgradesStatus{}

	// Is the package installed?
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "dpkg", "-s", "unattended-upgrades").Output()
	if err != nil || !strings.Contains(string(out), "Status: install ok installed") {
		return status // not installed — all fields remain false/zero
	}
	status.Installed = true

	// Is UU enabled? The apt periodic config is the source of truth for periodic runs.
	// systemctl is-enabled is unreliable: it can return "static"/"enabled-runtime" even
	// when UU is fully active, or "enabled" even when the apt timer has been disabled.
	status.Enabled = readUUEnabledFromAptConfig()

	// Reboot required?
	if _, statErr := os.Stat(uuRebootRequired); statErr == nil {
		status.RebootRequired = true
	}

	// Read configuration
	status.Config = readUUConfig()

	// Read new log entries since last cursor position
	status.NewRuns = readNewUURuns()

	return status
}

// readUUEnabledFromAptConfig reads /etc/apt/apt.conf.d/20auto-upgrades to determine
// whether unattended-upgrades is enabled for periodic runs. Falls back to systemctl
// if the file is absent or doesn't contain the directive.
func readUUEnabledFromAptConfig() bool {
	data, err := os.ReadFile(uuAutoUpgradesConf)
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "//") || line == "" {
				continue
			}
			if strings.Contains(line, "Unattended-Upgrade") {
				return strings.Contains(line, `"1"`)
			}
		}
	}
	// File absent or directive missing: fall back to systemctl is-enabled.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "systemctl", "is-enabled", "unattended-upgrades").Output()
	state := strings.TrimSpace(string(out))
	return state == "enabled" || state == "enabled-runtime"
}

// readUUConfig parses the apt config files to extract essential settings.
func readUUConfig() UUConfig {
	cfg := UUConfig{
		SecurityOnly:   true, // default: security only
		AutoRebootTime: "02:00",
	}

	data, err := os.ReadFile(uuMainConf)
	if err != nil {
		return cfg
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.Contains(line, `"${distro_id}:${distro_codename}"`) ||
			strings.Contains(line, `"${distro_id}:${distro_codename}-updates"`) {
			cfg.SecurityOnly = false
		}
		if strings.Contains(line, `Unattended-Upgrade::Automatic-Reboot "true"`) {
			cfg.AutoReboot = true
		}
		if strings.Contains(line, "Unattended-Upgrade::Automatic-Reboot-Time") {
			parts := strings.SplitN(line, `"`, 3)
			if len(parts) >= 3 {
				cfg.AutoRebootTime = parts[1]
			}
		}
		if strings.Contains(line, `Unattended-Upgrade::Remove-Unused-Dependencies "true"`) ||
			strings.Contains(line, `Unattended-Upgrade::Remove-Unused-Kernel-Packages "true"`) {
			cfg.RemoveUnused = true
		}
	}
	return cfg
}

// readNewUURuns reads the UU log file from the cursor position and returns newly completed runs.
func readNewUURuns() []UURun {
	uuLogMu.Lock()
	defer uuLogMu.Unlock()

	f, err := os.Open(uuLogPath)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil
	}
	fileSize := info.Size()

	// First call: read a limited tail window for history, then initialise cursor to EOF.
	if uuLogCursor == -1 {
		startPos := int64(0)
		if fileSize > uuInitialBackfillBytes {
			startPos = fileSize - uuInitialBackfillBytes
		}
		if _, err := f.Seek(startPos, io.SeekStart); err != nil {
			return nil
		}
		scanner := bufio.NewScanner(f)
		if startPos > 0 {
			// Skip partial line when starting mid-file.
			scanner.Scan()
		}
		runs := parseUURuns(scanner)
		if len(runs) > uuInitialMaxRuns {
			runs = runs[len(runs)-uuInitialMaxRuns:]
		}
		uuLogCursor = fileSize
		return runs
	}

	// File was rotated (smaller than cursor) — reset to beginning.
	if fileSize < uuLogCursor {
		uuLogCursor = 0
	}

	if fileSize == uuLogCursor {
		return nil
	}

	if _, err := f.Seek(uuLogCursor, io.SeekStart); err != nil {
		return nil
	}

	scanner := bufio.NewScanner(f)
	runs := parseUURuns(scanner)

	// Update cursor to current file position
	if pos, err := f.Seek(0, io.SeekCurrent); err == nil {
		uuLogCursor = pos
	}

	return runs
}

func parseUURuns(scanner *bufio.Scanner) []UURun {
	var runs []UURun
	var current *UURun
	var snippetLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "INFO Starting unattended upgrades script") {
			ts := parseUUTimestamp(line)
			current = &UURun{RunAt: ts}
			snippetLines = []string{line}
			continue
		}
		if current == nil {
			continue
		}

		snippetLines = append(snippetLines, line)
		if len(snippetLines) > 30 {
			snippetLines = snippetLines[1:]
		}

		if strings.Contains(line, "INFO Packages that will be upgraded:") {
			idx := strings.Index(line, "Packages that will be upgraded:")
			pkgStr := strings.TrimSpace(line[idx+len("Packages that will be upgraded:"):])
			if pkgStr != "" {
				current.Packages = strings.Fields(pkgStr)
			}
		}
		if strings.Contains(line, "ERROR") {
			current.HadError = true
		}
		if strings.Contains(line, "All upgrades installed") ||
			strings.Contains(line, "No packages found that can be upgraded") ||
			strings.Contains(line, "Packages that are not upgraded") {
			if len(snippetLines) > 0 {
				current.LogSnippet = strings.Join(snippetLines, "\n")
			}
			runs = append(runs, *current)
			current = nil
			snippetLines = nil
		}
	}

	return runs
}

// parseUUTimestamp extracts the timestamp from a UU log line.
// Format: "2006-01-02 15:04:05,000 INFO ..."
func parseUUTimestamp(line string) time.Time {
	if len(line) < 19 {
		return time.Now()
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", line[:19], time.Local)
	if err != nil {
		return time.Now()
	}
	return t
}

// writeUUConfig writes the two unattended-upgrades config files based on the given UUConfig.
// It preserves the current enabled/disabled state in 20auto-upgrades so that configure_uu
// does not accidentally re-enable UU when toggle_uu previously disabled it.
func writeUUConfig(cfg UUConfig) error {
	// Preserve the current Unattended-Upgrade value; default to "1" on first write.
	enabledVal := "1"
	if data, err := os.ReadFile(uuAutoUpgradesConf); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "//") || line == "" {
				continue
			}
			if strings.Contains(line, "Unattended-Upgrade") {
				if strings.Contains(line, `"0"`) {
					enabledVal = "0"
				}
				break
			}
		}
	}
	autoUpgrades := "APT::Periodic::Update-Package-Lists \"1\";\n" +
		"APT::Periodic::Download-Upgradeable-Packages \"1\";\n" +
		"APT::Periodic::AutocleanInterval \"7\";\n" +
		"APT::Periodic::Unattended-Upgrade \"" + enabledVal + "\";\n"
	if err := os.WriteFile(uuAutoUpgradesConf, []byte(autoUpgrades), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", uuAutoUpgradesConf, err)
	}

	var b strings.Builder
	b.WriteString(`Unattended-Upgrade::Allowed-Origins {
	"${distro_id}:${distro_codename}-security";
`)
	if !cfg.SecurityOnly {
		b.WriteString(`	"${distro_id}:${distro_codename}";
	"${distro_id}:${distro_codename}-updates";
`)
	}
	b.WriteString("};\n\n")

	autoReboot := "false"
	if cfg.AutoReboot {
		autoReboot = "true"
	}
	b.WriteString(fmt.Sprintf("Unattended-Upgrade::Automatic-Reboot \"%s\";\n", autoReboot))

	rebootTime := cfg.AutoRebootTime
	if rebootTime == "" {
		rebootTime = "02:00"
	}
	b.WriteString(fmt.Sprintf("Unattended-Upgrade::Automatic-Reboot-Time \"%s\";\n", rebootTime))

	removeUnused := "false"
	if cfg.RemoveUnused {
		removeUnused = "true"
	}
	b.WriteString(fmt.Sprintf("Unattended-Upgrade::Remove-Unused-Dependencies \"%s\";\n", removeUnused))
	b.WriteString(fmt.Sprintf("Unattended-Upgrade::Remove-Unused-Kernel-Packages \"%s\";\n", removeUnused))

	if err := os.WriteFile(uuMainConf, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", uuMainConf, err)
	}
	return nil
}

// ToggleUnattendedUpgrades enables or disables unattended-upgrades.
// It updates the apt periodic config (the actual mechanism that schedules periodic runs)
// and also toggles the systemd service as a best-effort secondary action.
func ToggleUnattendedUpgrades(enable bool) (string, error) {
	val := "0"
	if enable {
		val = "1"
	}
	content := "APT::Periodic::Update-Package-Lists \"1\";\n" +
		"APT::Periodic::Download-Upgradeable-Packages \"1\";\n" +
		"APT::Periodic::AutocleanInterval \"7\";\n" +
		"APT::Periodic::Unattended-Upgrade \"" + val + "\";\n"
	if err := os.WriteFile(uuAutoUpgradesConf, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", uuAutoUpgradesConf, err)
	}
	// Best-effort: also toggle the systemd service (not all distros use it for scheduling).
	action := "disable"
	if enable {
		action = "enable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "systemctl", action, "--now", "unattended-upgrades").CombinedOutput()
	return string(out), nil
}

// ConfigureUnattendedUpgrades writes config files for unattended-upgrades.
func ConfigureUnattendedUpgrades(cfg UUConfig) error {
	return writeUUConfig(cfg)
}

// RunUnattendedUpgrades triggers a manual unattended-upgrade run with streaming output.
func RunUnattendedUpgrades(streamCallback func(string)) (string, error) {
	cmd := exec.Command("unattended-upgrade", "--verbose")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	slog.Info("executing unattended-upgrade --verbose")
	return runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)
}

// InstallUnattendedUpgrades installs the unattended-upgrades package if not present.
func InstallUnattendedUpgrades(streamCallback func(string)) (string, error) {
	cmd := exec.Command("apt-get", "install", "-y", "-q", "unattended-upgrades")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	slog.Info("executing apt-get install -y unattended-upgrades")
	return runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)
}
