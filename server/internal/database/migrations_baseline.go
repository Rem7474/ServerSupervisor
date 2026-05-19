package database

import (
	"bufio"
	"bytes"
	"embed"
	"regexp"
)

// baselineMarker matches the "-- ===== BEGIN <filename>.sql =====" header
// the baseline migration uses to declare which legacy migration files it
// subsumes. The migrate() function relies on this manifest to decide which
// migrations are safe to mark "applied" without executing on a fresh install.
var baselineMarker = regexp.MustCompile(`^--\s*=====\s*BEGIN\s+([^\s=]+\.sql)\s*=====`)

// readBaselineManifest scans the baseline migration file and returns the set
// of legacy migration filenames it embeds. Returning the set as a map gives
// O(1) lookups when the caller is checking which migrations to skip.
//
// On error the caller should fail the migration cycle rather than fall back
// to the old "mark everything applied" behaviour — that bug was the reason
// post-baseline migrations like 047_alert_rule_source_type.sql silently
// skipped on fresh installs.
func readBaselineManifest(fsys embed.FS, baselineName string) (map[string]struct{}, error) {
	data, err := fsys.ReadFile("migrations/" + baselineName)
	if err != nil {
		return nil, err
	}

	out := map[string]struct{}{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Some statements in the baseline (CHECK constraints, JSON literals) can
	// be longer than the default 64 KiB scanner buffer. Bump it generously.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		m := baselineMarker.FindStringSubmatch(line)
		if m != nil {
			out[m[1]] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
