package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type BotDetectionIP struct {
	IP          string   `json:"ip"`
	Hits        int      `json:"hits"`
	UniquePaths int      `json:"unique_paths"`
	LastSeen    string   `json:"last_seen"`
	UserAgents  []string `json:"user_agents"`
}

type BotDetectionPath struct {
	Path string `json:"path"`
	Hits int    `json:"hits"`
}

type BotDetectionSummary struct {
	LogFilesScanned     []string           `json:"log_files_scanned"`
	TailLinesPerFile    int                `json:"tail_lines_per_file"`
	TotalRequests       int                `json:"total_requests"`
	SuspiciousRequests  int                `json:"suspicious_requests"`
	UniqueSuspiciousIPs int                `json:"unique_suspicious_ips"`
	TopSuspiciousIPs    []BotDetectionIP   `json:"top_suspicious_ips"`
	TopSuspiciousPaths  []BotDetectionPath `json:"top_suspicious_paths"`
	CollectedAt         time.Time          `json:"collected_at"`
}

// Format NPM custom :
// [date] - - STATUS - METHOD SCHEME DOMAIN "PATH" [Client IP] [Length X] [Gzip -] [Sent-to X] "UA" "-"
// Exemple :
// [23/Mar/2026:10:41:25 +0000] - 200 200 - GET https remcorp.fr "/" [Client 79.127.182.179] [Length 16183] [Gzip -] [Sent-to 192.168.1.213] "Mozilla/5.0 ..." "-"
var accessLogRegex = regexp.MustCompile(
	`^\[[^\]]+\] - \S+ (\d{3}) - (\S+) \S+ \S+ "([^"]*)" \[Client ([^\]]+)\].*"([^"]*)" "-"$`,
)

var suspiciousPathNeedles = []string{
	"/.env", "/wp-admin", "/wp-login", "/xmlrpc.php", "/cgi-bin", "/phpmyadmin", "/pma",
	"/manager/html", "/actuator", "/.git", "/vendor/phpunit", "/solr", "/hudson", "/jenkins",
	"/autodiscover", "/owa", "/etc/passwd", "/bin/bash", "/struts", "/boaform", "/api/jsonws",
}

var suspiciousUANeedles = []string{
	"masscan", "nmap", "zgrab", "sqlmap", "nikto", "dirbuster", "gobuster", "wpscan", "acunetix", "nessus",
}

func CollectBotDetection(logPathGlobs []string, tailLines int, topN int) (*BotDetectionSummary, error) {
	if tailLines <= 0 {
		tailLines = 5000
	}
	if topN <= 0 {
		topN = 10
	}

	files := expandGlobs(logPathGlobs)
	summary := &BotDetectionSummary{
		LogFilesScanned:    files,
		TailLinesPerFile:   tailLines,
		TopSuspiciousIPs:   []BotDetectionIP{},
		TopSuspiciousPaths: []BotDetectionPath{},
		CollectedAt:        time.Now().UTC(),
	}

	ipHits := map[string]int{}
	ipPaths := map[string]map[string]struct{}{}
	ipUAs := map[string]map[string]struct{}{}
	pathHits := map[string]int{}

	for _, file := range files {
		lines, err := readLastLines(file, tailLines)
		if err != nil {
			continue
		}
		for _, line := range lines {
			ip, method, path, ua, ok := parseAccessLine(line)
			if !ok {
				continue
			}
			summary.TotalRequests++
			if !isSuspicious(method, path, ua) {
				continue
			}
			summary.SuspiciousRequests++
			ipHits[ip]++
			pathHits[path]++
			if ipPaths[ip] == nil {
				ipPaths[ip] = map[string]struct{}{}
			}
			ipPaths[ip][path] = struct{}{}
			if ua != "" {
				if ipUAs[ip] == nil {
					ipUAs[ip] = map[string]struct{}{}
				}
				ipUAs[ip][ua] = struct{}{}
			}
		}
	}

	summary.UniqueSuspiciousIPs = len(ipHits)
	for ip, hits := range ipHits {
		entry := BotDetectionIP{
			IP:          ip,
			Hits:        hits,
			UniquePaths: len(ipPaths[ip]),
			LastSeen:    summary.CollectedAt.Format(time.RFC3339),
			UserAgents:  mapKeys(ipUAs[ip]),
		}
		summary.TopSuspiciousIPs = append(summary.TopSuspiciousIPs, entry)
	}
	sort.Slice(summary.TopSuspiciousIPs, func(i, j int) bool {
		return summary.TopSuspiciousIPs[i].Hits > summary.TopSuspiciousIPs[j].Hits
	})
	if len(summary.TopSuspiciousIPs) > topN {
		summary.TopSuspiciousIPs = summary.TopSuspiciousIPs[:topN]
	}

	for path, hits := range pathHits {
		summary.TopSuspiciousPaths = append(summary.TopSuspiciousPaths, BotDetectionPath{Path: path, Hits: hits})
	}
	sort.Slice(summary.TopSuspiciousPaths, func(i, j int) bool {
		return summary.TopSuspiciousPaths[i].Hits > summary.TopSuspiciousPaths[j].Hits
	})
	if len(summary.TopSuspiciousPaths) > topN {
		summary.TopSuspiciousPaths = summary.TopSuspiciousPaths[:topN]
	}

	return summary, nil
}

func expandGlobs(globs []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, pattern := range globs {
		if pattern == "" {
			continue
		}
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			if _, err := os.Stat(pattern); err == nil {
				if _, ok := seen[pattern]; !ok {
					seen[pattern] = struct{}{}
					out = append(out, pattern)
				}
			}
			continue
		}
		for _, m := range matches {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

func readLastLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	ring := make([]string, 0, n)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if len(ring) < n {
			ring = append(ring, line)
			continue
		}
		copy(ring, ring[1:])
		ring[n-1] = line
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ring, nil
}

// parseAccessLine parse le format de log custom de Nginx Proxy Manager :
// [date] - STATUS_UPSTREAM STATUS - METHOD SCHEME DOMAIN "PATH" [Client IP] [Length X] [Gzip -] [Sent-to X] "UA" "-"
// Groupes : (1) status, (2) method, (3) path+query, (4) client_ip, (5) user_agent
func parseAccessLine(line string) (ip, method, path, ua string, ok bool) {
	m := accessLogRegex.FindStringSubmatch(line)
	if len(m) == 0 {
		return "", "", "", "", false
	}
	// m[1]=status, m[2]=method, m[3]=path, m[4]=ip, m[5]=ua
	method = strings.ToUpper(strings.TrimSpace(m[2]))
	path = strings.TrimSpace(m[3])
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	if path == "" {
		path = "/"
	}
	ip = strings.TrimSpace(m[4])
	ua = strings.ToLower(strings.TrimSpace(m[5]))
	return ip, method, path, ua, true
}

func isSuspicious(method, path, ua string) bool {
	pathLower := strings.ToLower(path)
	for _, needle := range suspiciousPathNeedles {
		if strings.Contains(pathLower, needle) {
			return true
		}
	}
	for _, needle := range suspiciousUANeedles {
		if strings.Contains(ua, needle) {
			return true
		}
	}
	switch method {
	case "OPTIONS", "PROPFIND", "TRACE", "CONNECT":
		return true
	}
	return false
}

func mapKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}
