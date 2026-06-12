package handlers

// Pure unit tests for security/parse helpers. These run everywhere (no Docker /
// no DB needed), unlike the testcontainers-backed integration tests.

import (
	"testing"
)

func TestShouldResolveDockerTag(t *testing.T) {
	truthy := []string{"", "latest", "LATEST", "v4", "4", "v4.4", "4.4"}
	for _, tag := range truthy {
		if !shouldResolveDockerTag(tag) {
			t.Errorf("shouldResolveDockerTag(%q) = false, want true (mutable/broad tag)", tag)
		}
	}
	falsy := []string{"v4.4.1", "4.4.1", "v1.2.3", "stable", "bookworm", "v4-rc1"}
	for _, tag := range falsy {
		if shouldResolveDockerTag(tag) {
			t.Errorf("shouldResolveDockerTag(%q) = true, want false (already-pinned tag)", tag)
		}
	}
}

func TestParseVMID(t *testing.T) {
	cases := map[string]int{
		"100": 100,
		"1":   1,
		"0":   0, // not positive
		"-5":  0, // negative
		"abc": 0, // non-numeric
		"":    0, // empty
		"12x": 0, // trailing garbage
	}
	for in, want := range cases {
		if got := parseVMID(in); got != want {
			t.Errorf("parseVMID(%q) = %d, want %d", in, got, want)
		}
	}
}
