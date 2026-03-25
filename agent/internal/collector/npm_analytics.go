package collector

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type NPMPathHit struct {
	Path string `json:"path"`
	Hits int    `json:"hits"`
}

type NPMDomainStat struct {
	Domain    string       `json:"domain"`
	Hits      int          `json:"hits"`
	Bytes     uint64       `json:"bytes"`
	Errors4xx int          `json:"errors_4xx"`
	Errors5xx int          `json:"errors_5xx"`
	TopPaths  []NPMPathHit `json:"top_paths"`
}

type NPMSummary struct {
	LogFilesScanned  []string        `json:"log_files_scanned"`
	TailLinesPerFile int             `json:"tail_lines_per_file"`
	TotalRequests    int             `json:"total_requests"`
	TotalBytes       uint64          `json:"total_bytes"`
	TopDomains       []NPMDomainStat `json:"top_domains"`
	CollectedAt      time.Time       `json:"collected_at"`
}

var npmAccessLogRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[[^\]]+\] "([A-Z]+) ([^\"]*?) HTTP/[^\"]*" (\d{3}) (\S+)(?: "[^\"]*" "[^\"]*")?(?: "([^\"]*)")?`)

func CollectNPMAnalytics(logPathGlobs []string, tailLines int, topN int) (*NPMSummary, error) {
	if tailLines <= 0 {
		tailLines = 5000
	}
	if topN <= 0 {
		topN = 10
	}

	files := expandGlobs(logPathGlobs)
	summary := &NPMSummary{
		LogFilesScanned:  files,
		TailLinesPerFile: tailLines,
		TopDomains:       []NPMDomainStat{},
		CollectedAt:      time.Now().UTC(),
	}

	domainHits := map[string]int{}
	domainBytes := map[string]uint64{}
	domain4xx := map[string]int{}
	domain5xx := map[string]int{}
	domainPaths := map[string]map[string]int{}

	for _, file := range files {
		lines, err := readLastLines(file, tailLines)
		if err != nil {
			continue
		}

		for _, line := range lines {
			domain, path, status, bytes, ok := parseNPMAccessLine(line)
			if !ok {
				continue
			}

			summary.TotalRequests++
			summary.TotalBytes += bytes

			domainHits[domain]++
			domainBytes[domain] += bytes
			if status >= 400 && status < 500 {
				domain4xx[domain]++
			}
			if status >= 500 {
				domain5xx[domain]++
			}
			if domainPaths[domain] == nil {
				domainPaths[domain] = map[string]int{}
			}
			domainPaths[domain][path]++
		}
	}

	for domain, hits := range domainHits {
		entry := NPMDomainStat{
			Domain:    domain,
			Hits:      hits,
			Bytes:     domainBytes[domain],
			Errors4xx: domain4xx[domain],
			Errors5xx: domain5xx[domain],
			TopPaths:  topPaths(domainPaths[domain], 5),
		}
		summary.TopDomains = append(summary.TopDomains, entry)
	}

	sort.Slice(summary.TopDomains, func(i, j int) bool {
		return summary.TopDomains[i].Hits > summary.TopDomains[j].Hits
	})
	if len(summary.TopDomains) > topN {
		summary.TopDomains = summary.TopDomains[:topN]
	}

	return summary, nil
}

func parseNPMAccessLine(line string) (domain, path string, status int, bytes uint64, ok bool) {
	m := npmAccessLogRegex.FindStringSubmatch(line)
	if len(m) == 0 {
		return "", "", 0, 0, false
	}

	status, _ = strconv.Atoi(m[4])
	if m[5] != "-" {
		bytes, _ = strconv.ParseUint(m[5], 10, 64)
	}

	path = strings.TrimSpace(m[3])
	if q := strings.IndexByte(path, '?'); q >= 0 {
		path = path[:q]
	}
	if path == "" {
		path = "/"
	}

	domain = strings.ToLower(strings.TrimSpace(m[6]))
	if domain == "" || domain == "-" {
		domain = "(unknown)"
	}

	return domain, path, status, bytes, true
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
