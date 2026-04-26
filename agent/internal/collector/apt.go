package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
		log.Printf("apt-get upgrade --simulate failed: %v", err)
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
		log.Printf("APT: %d upgradable packages (%d security, %d CVEs)", status.PendingPackages, status.SecurityUpdates, len(cveInfos))
	} else {
		log.Printf("APT: %d upgradable packages (%d security)", status.PendingPackages, status.SecurityUpdates)
	}
	return status, nil
}

// ExecuteAptCommand runs an apt command (update, upgrade, dist-upgrade)
func ExecuteAptCommand(command string) (string, error) {
	return ExecuteAptCommandWithStreaming(command, nil)
}

// ExecuteAptCommandWithStreaming runs an apt-get command with real-time output streaming
// streamCallback is called for each chunk of output (can be nil)
func ExecuteAptCommandWithStreaming(command string, streamCallback func(chunk string)) (string, error) {
	var cmd *exec.Cmd
	switch command {
	case "update":
		cmd = exec.Command("apt-get", "update")
	case "upgrade":
		cmd = exec.Command("apt-get", "upgrade", "-y", "-qq", "-o", "Dpkg::Options::=--force-confold")
	case "dist-upgrade":
		cmd = exec.Command("apt-get", "dist-upgrade", "-y", "-qq", "-o", "Dpkg::Options::=--force-confold")
	default:
		return "", fmt.Errorf("unknown apt command: %s", command)
	}

	// Set non-interactive environment for unattended operation
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")

	log.Printf("Executing: apt-get %s -y", command)

	output, err := runCommandWithStreaming(cmd, streamCallback, aptCommandIdleTimeout)

	if err != nil {
		return output, fmt.Errorf("apt-get %s failed: %w\nOutput: %s", command, err, output)
	}

	log.Printf("apt-get %s completed successfully", command)
	return output, nil
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
				log.Printf("CVE API: could not fetch %s: %v", cves[idx].ID, err)
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
