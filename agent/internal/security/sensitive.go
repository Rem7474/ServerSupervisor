package security

import "strings"

// sensitivePatterns is the single authoritative list of key-name fragments that
// indicate a sensitive value (password, token, credential, etc.).
// Both env-var filtering and YAML redaction use this list so additions are
// never missed in one path but not the other.
var sensitivePatterns = []string{
	"password", "secret", "token", "key", "pass",
	"pwd", "credential", "auth", "private", "salt",
	"api_key", "apikey", "bearer", "jwt",
	// "swift" covers OpenStack Swift env vars (e.g. SWIFT_API_KEY, ST_AUTH)
	// used by Restic's Swift/OpenStack storage backend; most are already
	// caught by "key"/"auth" above, this closes the remaining gap.
	"swift",
}

// IsEnvKeySensitive reports whether an environment variable name should be
// redacted before sending to the server.
func IsEnvKeySensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, p := range sensitivePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// FilterYAML redacts the value portion of any line whose key matches a
// sensitive pattern (matched as `key=` or `key:`).
func FilterYAML(input string) string {
	var out []string
	for _, line := range strings.Split(input, "\n") {
		lower := strings.ToLower(line)
		redact := false
		for _, p := range sensitivePatterns {
			if strings.Contains(lower, p+"=") || strings.Contains(lower, p+":") {
				redact = true
				break
			}
		}
		if redact {
			// Look for either separator — `:` (YAML) or `=` (shell env-style,
			// e.g. sourced resticconf output). The previous version only
			// handled `:`, silently leaving `KEY=value` lines unredacted even
			// though they were flagged as sensitive above.
			if idx := strings.IndexAny(line, ":="); idx >= 0 {
				out = append(out, line[:idx+1]+" [REDACTED]")
			} else {
				out = append(out, line)
			}
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
