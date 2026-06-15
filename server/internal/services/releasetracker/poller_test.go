package releasetracker

import "testing"

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
