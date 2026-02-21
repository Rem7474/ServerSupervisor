package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CVEInfo struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Package  string `json:"package"`
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

	// Check if apt is available
	if _, err := exec.LookPath("apt"); err != nil {
		return nil, fmt.Errorf("apt not found in PATH")
	}

	// Get last update time from apt history log
	status.LastUpdate = getLastAptAction("Start-Date", "/var/log/apt/history.log")
	status.LastUpgrade = getLastAptUpgrade()

	// List upgradable packages
	out, err := exec.CommandContext(ctx, "apt", "list", "--upgradable").Output()
	if err != nil {
		log.Printf("apt list --upgradable failed: %v", err)
		return status, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var packages []string
	secCount := 0
	var cveInfos []CVEInfo

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		// Format: package/suite version arch [upgradable from: version]
		parts := strings.SplitN(line, "/", 2)
		if len(parts) >= 1 {
			packageName := parts[0]
			packages = append(packages, packageName)

			// Check if it's a security update
			if strings.Contains(line, "-security") {
				secCount++
				// Extract CVEs for security packages only if requested
				if extractCVE {
					cves := extractCVEsForPackage(packageName)
					cveInfos = append(cveInfos, cves...)
				}
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

// ExecuteAptCommandWithStreaming runs an apt command with real-time output streaming
// streamCallback is called for each chunk of output (can be nil)
func ExecuteAptCommandWithStreaming(command string, streamCallback func(chunk string)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch command {
	case "update":
		cmd = exec.CommandContext(ctx, "apt", "update", "-y", "-qq")
	case "upgrade":
		cmd = exec.CommandContext(ctx, "apt", "upgrade", "-y", "-qq", "--allow-unauthenticated")
	case "dist-upgrade":
		cmd = exec.CommandContext(ctx, "apt", "dist-upgrade", "-y", "-qq", "--allow-unauthenticated")
	default:
		return "", fmt.Errorf("unknown apt command: %s", command)
	}

	log.Printf("Executing: apt %s -y", command)

	// If streaming callback provided, capture output in real-time
	if streamCallback != nil {
		output, err := runCommandWithStreaming(cmd, streamCallback)
		if err != nil {
			return output, fmt.Errorf("apt %s failed: %w\nOutput: %s", command, err, output)
		}
		log.Printf("apt %s completed successfully", command)
		return output, nil
	}

	// Fallback to simple execution
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		return output, fmt.Errorf("apt %s failed: %w\nOutput: %s", command, err, output)
	}

	log.Printf("apt %s completed successfully", command)
	return output, nil
}

// runCommandWithStreaming executes a command and streams output via callback
func runCommandWithStreaming(cmd *exec.Cmd, streamCallback func(chunk string)) (string, error) {
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

	// Read stdout and stderr concurrently - each goroutine has its own buffer
	done := make(chan error, 2)

	go func() {
		buf := make([]byte, 4096) // Local buffer for this goroutine
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				if streamCallback != nil {
					streamCallback(chunk)
				}
			}
			if err != nil {
				done <- nil
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096) // Local buffer for this goroutine
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				if streamCallback != nil {
					streamCallback(chunk)
				}
			}
			if err != nil {
				done <- nil
				return
			}
		}
	}()

	// Wait for both streams to finish
	<-done
	<-done

	err = cmd.Wait()
	return fullOutput.String(), err
}

func getLastAptAction(prefix, logFile string) time.Time {
	out, err := exec.Command("grep", prefix, logFile).Output()
	if err != nil {
		return time.Time{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return time.Time{}
	}
	// Get last entry
	lastLine := lines[len(lines)-1]
	parts := strings.SplitN(lastLine, ":", 2)
	if len(parts) != 2 {
		return time.Time{}
	}
	ts := strings.TrimSpace(parts[1])
	t, _ := time.Parse("2006-01-02  15:04:05", ts)
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
	return time.Unix(epoch, 0)
}

// extractCVEsForPackage extracts CVE information from package changelog and USN
func extractCVEsForPackage(packageName string) []CVEInfo {
	var cves []CVEInfo
	cveMap := make(map[string]bool) // To avoid duplicates

	// Try to get changelog (with timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "apt-get", "changelog", packageName)
	output, err := cmd.Output()
	if err != nil {
		// If changelog fails, try apt-cache policy
		return extractCVEsFromPolicy(packageName)
	}

	changelog := string(output)

	// Extract CVE IDs using regex (CVE-YYYY-NNNNN format)
	cveRegex := regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)
	cveIDs := cveRegex.FindAllString(changelog, -1)

	// Extract severity from USN (Ubuntu Security Notice) format
	// Format: * SECURITY UPDATE: ... (LP: #NNNNNN, CVE-XXXX-YYYY)
	// or: - debian/patches/CVE-XXXX-YYYY.patch: fix ...
	severityRegex := regexp.MustCompile(`(?i)(critical|high|medium|low|negligible).*?(CVE-\d{4}-\d{4,7})`)
	severityMatches := severityRegex.FindAllStringSubmatch(changelog, -1)

	severityMap := make(map[string]string)
	for _, match := range severityMatches {
		if len(match) >= 3 {
			severity := strings.ToUpper(match[1])
			cveID := match[2]
			severityMap[cveID] = severity
		}
	}

	// Build CVE list with severity
	for _, cveID := range cveIDs {
		if cveMap[cveID] {
			continue // Skip duplicates
		}
		cveMap[cveID] = true

		severity := severityMap[cveID]
		if severity == "" {
			severity = detectSeverityFromChangelog(changelog, cveID)
		}

		cves = append(cves, CVEInfo{
			ID:       cveID,
			Severity: severity,
			Package:  packageName,
		})
	}

	return cves
}

// extractCVEsFromPolicy extracts CVE info from apt-cache policy (fallback method)
func extractCVEsFromPolicy(packageName string) []CVEInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "apt-cache", "policy", packageName)
	output, err := cmd.Output()
	if err != nil {
		return []CVEInfo{}
	}

	policy := string(output)
	var cves []CVEInfo
	cveMap := make(map[string]bool)

	// Look for security repositories in policy output
	// Format: *** 1.2.3ubuntu1.1 500
	//         500 http://security.ubuntu.com/ubuntu focal-security/main amd64 Packages
	if strings.Contains(policy, "security.ubuntu.com") || strings.Contains(policy, "-security") {
		// This is a security update, but we can't extract specific CVEs from policy
		// Return a generic CVE marker
		cves = append(cves, CVEInfo{
			ID:       "SECURITY-UPDATE",
			Severity: "UNKNOWN",
			Package:  packageName,
		})
	}

	cveRegex := regexp.MustCompile(`CVE-\d{4}-\d{4,7}`)
	cveIDs := cveRegex.FindAllString(policy, -1)
	for _, cveID := range cveIDs {
		if !cveMap[cveID] {
			cveMap[cveID] = true
			cves = append(cves, CVEInfo{
				ID:       cveID,
				Severity: "MEDIUM", // Default severity when unknown
				Package:  packageName,
			})
		}
	}

	return cves
}

// detectSeverityFromChangelog tries to infer severity from changelog context
func detectSeverityFromChangelog(changelog, cveID string) string {
	// Find the CVE in changelog and look for severity keywords nearby
	cveIndex := strings.Index(changelog, cveID)
	if cveIndex == -1 {
		return "MEDIUM" // Default
	}

	// Extract 500 chars before and after CVE mention
	start := cveIndex - 500
	if start < 0 {
		start = 0
	}
	end := cveIndex + 500
	if end > len(changelog) {
		end = len(changelog)
	}
	context := strings.ToLower(changelog[start:end])

	// Look for severity keywords in context
	if strings.Contains(context, "critical") {
		return "CRITICAL"
	}
	if strings.Contains(context, "high") || strings.Contains(context, "important") {
		return "HIGH"
	}
	if strings.Contains(context, "low") || strings.Contains(context, "negligible") {
		return "LOW"
	}
	if strings.Contains(context, "medium") || strings.Contains(context, "moderate") {
		return "MEDIUM"
	}

	// Check for severity indicators
	if strings.Contains(context, "remote code execution") || strings.Contains(context, "privilege escalation") {
		return "CRITICAL"
	}
	if strings.Contains(context, "denial of service") || strings.Contains(context, "information disclosure") {
		return "MEDIUM"
	}

	return "MEDIUM" // Default when we can't determine
}
