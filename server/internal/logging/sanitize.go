package logging

import "strings"

// Sanitize strips CR and LF characters from s before logging so untrusted or
// externally-sourced values cannot forge additional log records (CWE-117).
func Sanitize(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", "")
}
