package api

// Pure unit test for sensitive query-param masking in access logs (no DB needed).

import (
	"strings"
	"testing"
)

func TestMaskSensitiveParams(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		masked  []string // substrings that MUST appear
		leaked  []string // substrings that MUST NOT appear
	}{
		{
			name:   "token masked",
			query:  "token=supersecret&page=2",
			masked: []string{"token=***MASKED***", "page=2"},
			leaked: []string{"supersecret"},
		},
		{
			name:   "password and api_key masked",
			query:  "password=hunter2&api_key=abc123",
			masked: []string{"password=***MASKED***", "api_key=***MASKED***"},
			leaked: []string{"hunter2", "abc123"},
		},
		{
			name:   "case-insensitive key match",
			query:  "Token=xyz&Secret=zzz",
			masked: []string{"***MASKED***"},
			leaked: []string{"xyz", "zzz"},
		},
		{
			name:   "non-sensitive params untouched",
			query:  "host=h1&limit=50",
			masked: []string{"host=h1", "limit=50"},
			leaked: []string{"MASKED"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maskSensitiveParams(c.query)
			for _, want := range c.masked {
				if !strings.Contains(got, want) {
					t.Errorf("maskSensitiveParams(%q) = %q, missing %q", c.query, got, want)
				}
			}
			for _, bad := range c.leaked {
				if strings.Contains(got, bad) {
					t.Errorf("maskSensitiveParams(%q) = %q, leaked %q", c.query, got, bad)
				}
			}
		})
	}
}
