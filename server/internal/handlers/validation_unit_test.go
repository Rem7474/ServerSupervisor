package handlers

// Pure unit tests for security/parse helpers. These run everywhere (no Docker /
// no DB needed), unlike the testcontainers-backed integration tests.

import (
	"runtime"
	"testing"
)

func TestIsValidWorkingDir(t *testing.T) {
	// Note: isValidWorkingDir uses filepath.IsAbs, whose semantics are
	// OS-specific. The server runs on Linux (Docker) and validates paths
	// destined for a Linux agent, so absolute-path cases are only asserted on
	// Linux. The relative-path rejection below is portable and is the core of
	// the path-traversal guard.
	portable := []struct {
		in   string
		want bool
	}{
		{"", true},               // empty allowed (agent uses its default)
		{"relative/path", false}, // not absolute → rejected everywhere
		{"./app", false},         // not absolute
		{"../etc", false},        // relative traversal
		{"a/../b", false},        // relative, even after cleaning
	}
	for _, c := range portable {
		if got := isValidWorkingDir(c.in); got != c.want {
			t.Errorf("isValidWorkingDir(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	if runtime.GOOS == "linux" {
		linuxCases := []struct {
			in   string
			want bool
		}{
			{"/opt/app", true},        // absolute, clean
			{"/srv/stacks/web", true}, // absolute, nested
		}
		for _, c := range linuxCases {
			if got := isValidWorkingDir(c.in); got != c.want {
				t.Errorf("isValidWorkingDir(%q) = %v, want %v (linux)", c.in, got, c.want)
			}
		}
	}
}

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
		"100":  100,
		"1":    1,
		"0":    0,  // not positive
		"-5":   0,  // negative
		"abc":  0,  // non-numeric
		"":     0,  // empty
		"12x":  0,  // trailing garbage
	}
	for in, want := range cases {
		if got := parseVMID(in); got != want {
			t.Errorf("parseVMID(%q) = %d, want %d", in, got, want)
		}
	}
}
