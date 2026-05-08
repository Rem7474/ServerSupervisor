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
			if idx := strings.Index(line, ":"); idx >= 0 {
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
