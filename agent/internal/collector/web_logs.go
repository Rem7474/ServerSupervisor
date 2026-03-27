package collector

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type WebRequest struct {
	Timestamp string `json:"timestamp"`
	IP        string `json:"ip"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Bytes     int64  `json:"bytes"`
	UserAgent string `json:"user_agent"`
	Domain    string `json:"domain"`
	Category  string `json:"category,omitempty"`
}

type NPMPathHit struct {
	Path string `json:"path"`
	Hits int    `json:"hits"`
}

type NPMDomainStat struct {
	Domain    string         `json:"domain"`
	Hits      int            `json:"hits"`
	Bytes     int64          `json:"bytes"`
	Errors4xx int            `json:"errors_4xx"`
	Errors5xx int            `json:"errors_5xx"`
	Methods   map[string]int `json:"methods"`
	TopPaths  []NPMPathHit   `json:"top_paths"`
}

type TrafficSummary struct {
	TotalRequests int             `json:"total_requests"`
	TotalBytes    int64           `json:"total_bytes"`
	Errors4xx     int             `json:"errors_4xx"`
	Errors5xx     int             `json:"errors_5xx"`
	TopDomains    []NPMDomainStat `json:"top_domains"`
}

type BotDetectionIP struct {
	IP          string       `json:"ip"`
	Hits        int          `json:"hits"`
	UniquePaths int          `json:"unique_paths"`
	FirstSeen   string       `json:"first_seen"`
	LastSeen    string       `json:"last_seen"`
	Category    string       `json:"category"`
	UserAgents  []string     `json:"user_agents"`
	Requests    []WebRequest `json:"requests"`
}

type BotDetectionPath struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Hits     int    `json:"hits"`
}

type ThreatSummary struct {
	SuspiciousRequests  int                `json:"suspicious_requests"`
	UniqueSuspiciousIPs int                `json:"unique_suspicious_ips"`
	TopSuspiciousIPs    []BotDetectionIP   `json:"top_suspicious_ips"`
	TopSuspiciousPaths  []BotDetectionPath `json:"top_suspicious_paths"`
}

type WebLogReport struct {
	Source           string          `json:"source"`
	Traffic          *TrafficSummary `json:"traffic"`
	Threats          *ThreatSummary  `json:"threats"`
	Requests         []WebRequest    `json:"requests"`
	LogFilesScanned  []string        `json:"log_files_scanned"`
	TailLinesPerFile int             `json:"tail_lines_per_file"`
	TotalRequests    int             `json:"total_requests"`
	CollectedAt      time.Time       `json:"collected_at"`
}

type parsedLine struct {
	timestamp time.Time
	ip        string
	method    string
	path      string
	status    int
	bytes     int64
	ua        string
	domain    string
	source    string
}

type webLogCursorState struct {
	Files map[string]webLogCursorEntry `json:"files"`
}

type webLogCursorEntry struct {
	Offset         int64     `json:"offset"`
	Size           int64     `json:"size"`
	FileModUnix    int64     `json:"file_mod_unix,omitempty"`
	BackfillOffset int64     `json:"backfill_offset,omitempty"`
	BackfillLimit  int64     `json:"backfill_limit,omitempty"`
	BackfillDone   bool      `json:"backfill_done,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

var npmAccessLogRegex = regexp.MustCompile(
	`^\[([^\]]+)\] - \S+ (\d{3}) - (\S+) \S+ (\S+) "([^"]*)" \[Client ([^\]]+)\] \[Length (\d+)[^\]]*\].*"([^"]*)" "-"$`,
)

var commonAccessLogRegex = regexp.MustCompile(
	`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) ([^\s"]+) [^"]+" (\d{3}) (\d+|-) "[^"]*" "([^"]*)"`,
)

var suspiciousPathNeedles = []string{
	"/.env", "/wp-admin", "/wp-login", "/xmlrpc.php", "/cgi-bin", "/phpmyadmin", "/pma",
	"/manager/html", "/actuator", "/.git", "/vendor/phpunit", "/solr", "/hudson", "/jenkins",
	"/autodiscover", "/owa", "../", "/etc/passwd", "/bin/bash", "/struts", "/boaform", "/api/jsonws",
}

var suspiciousUANeedles = []string{
	"masscan", "nmap", "zgrab", "sqlmap", "nikto", "dirbuster", "gobuster", "wpscan", "acunetix", "nessus",
}

func CollectWebLogs(logPathGlobs []string, tailLines int, topN int, requestLimit int, cursorFile string) (*WebLogReport, error) {
	if tailLines <= 0 {
		tailLines = 5000
	}
	if topN <= 0 {
		topN = 10
	}
	if requestLimit <= 0 {
		requestLimit = 200
	}

	files := expandGlobs(logPathGlobs)
	now := time.Now().UTC()
	cursor := loadWebLogCursor(cursorFile)
	seenFiles := map[string]struct{}{}
	report := &WebLogReport{
		Source:           "unknown",
		Traffic:          &TrafficSummary{TopDomains: []NPMDomainStat{}},
		Threats:          &ThreatSummary{TopSuspiciousIPs: []BotDetectionIP{}, TopSuspiciousPaths: []BotDetectionPath{}},
		Requests:         make([]WebRequest, 0, requestLimit),
		LogFilesScanned:  files,
		TailLinesPerFile: tailLines,
		CollectedAt:      now,
	}

	domainHits := map[string]int{}
	domainBytes := map[string]int64{}
	domain4xx := map[string]int{}
	domain5xx := map[string]int{}
	domainMethods := map[string]map[string]int{}
	domainPaths := map[string]map[string]int{}
	sourceHits := map[string]int{}

	ipHits := map[string]int{}
	ipPaths := map[string]map[string]struct{}{}
	ipUAs := map[string]map[string]struct{}{}
	ipReq := map[string][]WebRequest{}
	ipFirstSeen := map[string]time.Time{}
	ipLastSeen := map[string]time.Time{}
	ipCategory := map[string]string{}
	pathHits := map[string]int{}
	pathCategory := map[string]string{}

	for _, file := range files {
		seenFiles[file] = struct{}{}
		entry, hasEntry := cursor.Files[file]
		lines, nextEntry, err := readLinesForFile(file, tailLines, entry, hasEntry)
		if err != nil {
			continue
		}
		cursor.Files[file] = nextEntry
		for _, line := range lines {
			e, ok := parseAccessLine(line)
			if !ok {
				continue
			}
			sourceHits[e.source]++
			report.TotalRequests++
			report.Traffic.TotalRequests++
			report.Traffic.TotalBytes += e.bytes
			if e.status >= 400 && e.status < 500 {
				report.Traffic.Errors4xx++
			}
			if e.status >= 500 {
				report.Traffic.Errors5xx++
			}

			domain := strings.ToLower(strings.TrimSpace(e.domain))
			if domain == "" || domain == "-" {
				domain = "(unknown)"
			}
			domainHits[domain]++
			domainBytes[domain] += e.bytes
			if e.status >= 400 && e.status < 500 {
				domain4xx[domain]++
			}
			if e.status >= 500 {
				domain5xx[domain]++
			}
			if domainMethods[domain] == nil {
				domainMethods[domain] = map[string]int{}
			}
			domainMethods[domain][e.method]++
			if domainPaths[domain] == nil {
				domainPaths[domain] = map[string]int{}
			}
			domainPaths[domain][e.path]++

			category := suspiciousCategory(e.method, e.path, e.ua)
			request := WebRequest{
				Timestamp: e.timestamp.Format(time.RFC3339),
				IP:        e.ip,
				Method:    e.method,
				Path:      e.path,
				Status:    e.status,
				Bytes:     e.bytes,
				UserAgent: e.ua,
				Domain:    domain,
				Category:  category,
			}
			if len(report.Requests) < requestLimit {
				report.Requests = append(report.Requests, request)
			}

			if category == "" {
				continue
			}
			report.Threats.SuspiciousRequests++
			ipHits[e.ip]++
			pathHits[e.path]++
			pathCategory[e.path] = category
			if ipPaths[e.ip] == nil {
				ipPaths[e.ip] = map[string]struct{}{}
			}
			ipPaths[e.ip][e.path] = struct{}{}
			if ipUAs[e.ip] == nil {
				ipUAs[e.ip] = map[string]struct{}{}
			}
			if e.ua != "" {
				ipUAs[e.ip][e.ua] = struct{}{}
			}
			if _, ok := ipFirstSeen[e.ip]; !ok || e.timestamp.Before(ipFirstSeen[e.ip]) {
				ipFirstSeen[e.ip] = e.timestamp
			}
			if e.timestamp.After(ipLastSeen[e.ip]) {
				ipLastSeen[e.ip] = e.timestamp
			}
			if ipCategory[e.ip] == "" {
				ipCategory[e.ip] = category
			}
			if len(ipReq[e.ip]) < 20 {
				ipReq[e.ip] = append(ipReq[e.ip], request)
			}
		}
	}

	report.Threats.UniqueSuspiciousIPs = len(ipHits)
	for domain, hits := range domainHits {
		report.Traffic.TopDomains = append(report.Traffic.TopDomains, NPMDomainStat{
			Domain:    domain,
			Hits:      hits,
			Bytes:     domainBytes[domain],
			Errors4xx: domain4xx[domain],
			Errors5xx: domain5xx[domain],
			Methods:   domainMethods[domain],
			TopPaths:  topPaths(domainPaths[domain], 5),
		})
	}
	sort.Slice(report.Traffic.TopDomains, func(i, j int) bool {
		return report.Traffic.TopDomains[i].Hits > report.Traffic.TopDomains[j].Hits
	})
	if len(report.Traffic.TopDomains) > topN {
		report.Traffic.TopDomains = report.Traffic.TopDomains[:topN]
	}

	for ip, hits := range ipHits {
		report.Threats.TopSuspiciousIPs = append(report.Threats.TopSuspiciousIPs, BotDetectionIP{
			IP:          ip,
			Hits:        hits,
			UniquePaths: len(ipPaths[ip]),
			FirstSeen:   ipFirstSeen[ip].Format(time.RFC3339),
			LastSeen:    ipLastSeen[ip].Format(time.RFC3339),
			Category:    ipCategory[ip],
			UserAgents:  mapKeys(ipUAs[ip]),
			Requests:    ipReq[ip],
		})
	}
	sort.Slice(report.Threats.TopSuspiciousIPs, func(i, j int) bool {
		return report.Threats.TopSuspiciousIPs[i].Hits > report.Threats.TopSuspiciousIPs[j].Hits
	})
	if len(report.Threats.TopSuspiciousIPs) > topN {
		report.Threats.TopSuspiciousIPs = report.Threats.TopSuspiciousIPs[:topN]
	}

	for path, hits := range pathHits {
		report.Threats.TopSuspiciousPaths = append(report.Threats.TopSuspiciousPaths, BotDetectionPath{Path: path, Category: pathCategory[path], Hits: hits})
	}
	sort.Slice(report.Threats.TopSuspiciousPaths, func(i, j int) bool {
		return report.Threats.TopSuspiciousPaths[i].Hits > report.Threats.TopSuspiciousPaths[j].Hits
	})
	if len(report.Threats.TopSuspiciousPaths) > topN {
		report.Threats.TopSuspiciousPaths = report.Threats.TopSuspiciousPaths[:topN]
	}

	if report.Source == "unknown" {
		report.Source = dominantSource(sourceHits)
	}

	for file := range cursor.Files {
		if _, ok := seenFiles[file]; !ok {
			delete(cursor.Files, file)
		}
	}
	saveWebLogCursor(cursorFile, cursor)

	return report, nil
}

func parseAccessLine(line string) (parsedLine, bool) {
	if m := npmAccessLogRegex.FindStringSubmatch(line); len(m) > 0 {
		status, _ := strconv.Atoi(m[2])
		bytes, _ := strconv.ParseInt(m[7], 10, 64)
		ts := parseTimeOrNow([]string{m[1]}, []string{"02/Jan/2006:15:04:05 -0700"})
		path := cleanPath(m[5])
		return parsedLine{
			timestamp: ts,
			ip:        strings.TrimSpace(m[6]),
			method:    strings.ToUpper(strings.TrimSpace(m[3])),
			path:      path,
			status:    status,
			bytes:     bytes,
			ua:        strings.TrimSpace(m[8]),
			domain:    strings.TrimSpace(m[4]),
			source:    "npm",
		}, true
	}

	if m := commonAccessLogRegex.FindStringSubmatch(line); len(m) > 0 {
		status, _ := strconv.Atoi(m[5])
		bytes := int64(0)
		if m[6] != "-" {
			bytes, _ = strconv.ParseInt(m[6], 10, 64)
		}
		ts := parseTimeOrNow([]string{m[2]}, []string{"02/Jan/2006:15:04:05 -0700"})
		path := cleanPath(m[4])
		source := "nginx"
		if strings.Contains(strings.ToLower(line), "apache") {
			source = "apache"
		}
		if strings.Contains(strings.ToLower(line), "caddy") {
			source = "caddy"
		}
		return parsedLine{
			timestamp: ts,
			ip:        strings.TrimSpace(m[1]),
			method:    strings.ToUpper(strings.TrimSpace(m[3])),
			path:      path,
			status:    status,
			bytes:     bytes,
			ua:        strings.TrimSpace(m[7]),
			domain:    "(unknown)",
			source:    source,
		}, true
	}

	return parsedLine{}, false
}

func suspiciousCategory(method, path, ua string) string {
	pathLower := strings.ToLower(path)
	switch {
	case strings.Contains(pathLower, "/wp-") || strings.Contains(pathLower, "/xmlrpc.php"):
		return "WordPress"
	case strings.Contains(pathLower, "/admin") || strings.Contains(pathLower, "/manager/html") || strings.Contains(pathLower, "/phpmyadmin"):
		return "AdminPanel"
	case strings.Contains(pathLower, "../") || strings.Contains(pathLower, "/etc/passwd") || strings.Contains(pathLower, "/bin/bash"):
		return "PathTraversal"
	}

	for _, needle := range suspiciousPathNeedles {
		if strings.Contains(pathLower, needle) {
			return "KnownScanner"
		}
	}

	uaLower := strings.ToLower(ua)
	for _, needle := range suspiciousUANeedles {
		if strings.Contains(uaLower, needle) {
			return "KnownScanner"
		}
	}

	switch strings.ToUpper(method) {
	case "OPTIONS", "PROPFIND", "TRACE", "CONNECT":
		return "SuspiciousMethod"
	}

	return ""
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	if path == "" {
		return "/"
	}
	return path
}

func parseTimeOrNow(values []string, layouts []string) time.Time {
	for _, v := range values {
		for _, layout := range layouts {
			if ts, err := time.Parse(layout, strings.TrimSpace(v)); err == nil {
				return ts.UTC()
			}
		}
	}
	return time.Now().UTC()
}

func dominantSource(sourceHits map[string]int) string {
	if len(sourceHits) == 0 {
		return "unknown"
	}
	maxHits := -1
	best := "unknown"
	for source, hits := range sourceHits {
		if hits > maxHits {
			maxHits = hits
			best = source
		}
	}
	return best
}

func topPaths(paths map[string]int, topN int) []NPMPathHit {
	if len(paths) == 0 {
		return []NPMPathHit{}
	}
	out := make([]NPMPathHit, 0, len(paths))
	for path, hits := range paths {
		out = append(out, NPMPathHit{Path: path, Hits: hits})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Hits > out[j].Hits
	})
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

func expandGlobs(globs []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	appendMatch := func(path string) {
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}

	appendGlobMatches := func(pattern string) {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return
		}
		for _, m := range matches {
			appendMatch(m)
		}
	}

	for _, pattern := range globs {
		if pattern == "" {
			continue
		}
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			if _, err := os.Stat(pattern); err == nil {
				appendMatch(pattern)
				appendGlobMatches(pattern + ".*.gz")
			}
			continue
		}
		for _, m := range matches {
			appendMatch(m)
		}
		appendGlobMatches(pattern + ".*.gz")
	}
	sort.Strings(out)
	return out
}

func readLinesForFile(path string, maxLines int, prev webLogCursorEntry, hasPrev bool) ([]string, webLogCursorEntry, error) {
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		return readCompressedLines(path, prev, hasPrev)
	}
	return readIncrementalLines(path, maxLines, prev, hasPrev)
}

func readCompressedLines(path string, prev webLogCursorEntry, hasPrev bool) ([]string, webLogCursorEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, prev, err
	}

	modUnix := info.ModTime().UTC().Unix()
	next := webLogCursorEntry{
		Offset:       info.Size(),
		Size:         info.Size(),
		FileModUnix:  modUnix,
		BackfillDone: true,
		UpdatedAt:    time.Now().UTC(),
	}

	if hasPrev && prev.Size == info.Size() && prev.FileModUnix == modUnix {
		return []string{}, next, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, next, err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, next, err
	}
	defer func() { _ = gz.Close() }()

	lines := make([]string, 0, 4096)
	s := bufio.NewScanner(gz)
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 4*1024*1024)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return nil, next, err
	}

	return lines, next, nil
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

func readIncrementalLines(path string, maxLines int, prev webLogCursorEntry, hasPrev bool) ([]string, webLogCursorEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, prev, err
	}

	next := webLogCursorEntry{
		Offset:         info.Size(),
		Size:           info.Size(),
		FileModUnix:    info.ModTime().UTC().Unix(),
		BackfillOffset: prev.BackfillOffset,
		BackfillLimit:  prev.BackfillLimit,
		BackfillDone:   prev.BackfillDone,
		UpdatedAt:      time.Now().UTC(),
	}

	bootstrap := func() ([]string, webLogCursorEntry, error) {
		tailLines, tailStartOffset, err := readLastLinesWithStart(path, maxLines)
		if err != nil {
			return nil, next, err
		}
		next.Offset = info.Size()
		next.Size = info.Size()
		next.BackfillOffset = 0
		next.BackfillLimit = tailStartOffset
		next.BackfillDone = tailStartOffset <= 0

		if !next.BackfillDone {
			backfillLines, backfillOffset, done, err := readBackfillChunk(path, next.BackfillOffset, next.BackfillLimit, maxLines)
			if err == nil {
				tailLines = append(tailLines, backfillLines...)
				next.BackfillOffset = backfillOffset
				next.BackfillDone = done
			}
		}

		return tailLines, next, nil
	}

	if !hasPrev {
		return bootstrap()
	}

	if info.Size() < prev.Offset {
		// Log rotated or truncated: bootstrap from tail again.
		return bootstrap()
	}

	if next.BackfillLimit == 0 && next.BackfillOffset == 0 && !next.BackfillDone {
		// Compatibility for legacy cursors: backfill older content up to previous live offset.
		next.BackfillLimit = prev.Offset
		next.BackfillDone = next.BackfillLimit <= 0
	}

	out := make([]string, 0, maxLines*2)

	if info.Size() > prev.Offset {
		// Always keep live tail fresh by processing new appended lines.
		liveLines, err := readNewLinesFromOffset(path, prev.Offset, 0)
		if err != nil {
			return nil, next, err
		}
		out = append(out, liveLines...)
	}

	if !next.BackfillDone {
		// Progressively replay old history from the beginning in fixed chunks.
		backfillLines, backfillOffset, done, err := readBackfillChunk(path, next.BackfillOffset, next.BackfillLimit, maxLines)
		if err == nil {
			out = append(out, backfillLines...)
			next.BackfillOffset = backfillOffset
			next.BackfillDone = done
		}
	}

	return out, next, nil
}

func readBackfillChunk(path string, startOffset int64, stopOffset int64, maxLines int) ([]string, int64, bool, error) {
	if maxLines <= 0 {
		maxLines = 5000
	}
	if startOffset >= stopOffset {
		return []string{}, startOffset, true, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, startOffset, false, err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		return nil, startOffset, false, err
	}

	r := bufio.NewReader(f)
	lines := make([]string, 0, maxLines)
	cursor := startOffset
	for len(lines) < maxLines && cursor < stopOffset {
		raw, err := r.ReadString('\n')
		if len(raw) > 0 {
			nextCursor := cursor + int64(len(raw))
			if nextCursor > stopOffset {
				break
			}
			line := strings.TrimRight(raw, "\r\n")
			lines = append(lines, line)
			cursor = nextCursor
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, startOffset, false, err
		}
	}

	done := cursor >= stopOffset
	return lines, cursor, done, nil
}

func readLastLinesWithStart(path string, n int) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()

	type lineEntry struct {
		start int64
		line  string
	}

	ring := make([]lineEntry, 0, n)
	r := bufio.NewReader(f)
	var offset int64
	for {
		raw, err := r.ReadString('\n')
		if len(raw) > 0 {
			entry := lineEntry{start: offset, line: strings.TrimRight(raw, "\r\n")}
			if n > 0 {
				if len(ring) < n {
					ring = append(ring, entry)
				} else {
					copy(ring, ring[1:])
					ring[n-1] = entry
				}
			}
			offset += int64(len(raw))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
	}

	if len(ring) == 0 {
		return []string{}, offset, nil
	}

	lines := make([]string, 0, len(ring))
	for _, e := range ring {
		lines = append(lines, e.line)
	}
	return lines, ring[0].start, nil
}

func readNewLinesFromOffset(path string, offset int64, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	if maxLines <= 0 {
		lines := make([]string, 0, 1024)
		s := bufio.NewScanner(f)
		for s.Scan() {
			lines = append(lines, s.Text())
		}
		if err := s.Err(); err != nil {
			return nil, err
		}
		return lines, nil
	}

	ring := make([]string, 0, maxLines)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if len(ring) < maxLines {
			ring = append(ring, line)
			continue
		}
		copy(ring, ring[1:])
		ring[maxLines-1] = line
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ring, nil
}

func loadWebLogCursor(path string) *webLogCursorState {
	state := &webLogCursorState{Files: map[string]webLogCursorEntry{}}
	if strings.TrimSpace(path) == "" {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, state); err != nil {
		return &webLogCursorState{Files: map[string]webLogCursorEntry{}}
	}
	if state.Files == nil {
		state.Files = map[string]webLogCursorEntry{}
	}
	return state
}

func saveWebLogCursor(path string, state *webLogCursorState) {
	if strings.TrimSpace(path) == "" || state == nil {
		return
	}
	if state.Files == nil {
		state.Files = map[string]webLogCursorEntry{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
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
