package releasetracker

import "testing"

func TestNormalizeDigest(t *testing.T) {
	cases := map[string]string{
		"sha256:abc123": "abc123",
		"abc123":        "abc123",
		"":              "",
		"sha256:":       "",
		// Only the leading prefix is stripped, not a mid-string occurrence.
		"prefix-sha256:abc": "prefix-sha256:abc",
	}
	for in, want := range cases {
		if got := NormalizeDigest(in); got != want {
			t.Errorf("NormalizeDigest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v1.2.3": "1.2.3",
		"1.2.3":  "1.2.3",
		"v":      "",
		"":       "",
		// Only a leading lowercase "v" is stripped.
		"version": "ersion",
		"V1.2.3":  "V1.2.3",
	}
	for in, want := range cases {
		if got := NormalizeVersion(in); got != want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsVersionUpToDate(t *testing.T) {
	tests := []struct {
		name                                               string
		runningTag, runningDigest, latestTag, latestDigest string
		want                                               bool
	}{
		{
			name:       "equal digests (both prefixed) win regardless of tags",
			runningTag: "latest", runningDigest: "sha256:abc",
			latestTag: "latest", latestDigest: "sha256:abc",
			want: true,
		},
		{
			name:       "digest equality normalizes prefix on one side",
			runningTag: "v1.0.0", runningDigest: "abc",
			latestTag: "v2.0.0", latestDigest: "sha256:abc",
			want: true, // digest match short-circuits before tag comparison
		},
		{
			name:       "explicit equal tags with mixed v-prefix",
			runningTag: "v1.2.3", runningDigest: "",
			latestTag: "1.2.3", latestDigest: "",
			want: true,
		},
		{
			name:       "explicit differing tags, no digests",
			runningTag: "v1.2.3", runningDigest: "",
			latestTag: "v1.2.4", latestDigest: "",
			want: false,
		},
		{
			name:       "channel tag: running patch matches latest major channel",
			runningTag: "v5.13.2", runningDigest: "",
			latestTag: "v5", latestDigest: "",
			want: true,
		},
		{
			name:       "channel tag does not match a different major",
			runningTag: "v6.0.1", runningDigest: "",
			latestTag: "v5", latestDigest: "",
			want: false,
		},
		{
			name:       "channel prefix must be a dotted boundary (v51 != v5 channel)",
			runningTag: "v51.0.0", runningDigest: "",
			latestTag: "v5", latestDigest: "",
			want: false,
		},
		{
			name:       "both latest, equal digests",
			runningTag: "latest", runningDigest: "sha256:dead",
			latestTag: "latest", latestDigest: "sha256:dead",
			want: true,
		},
		{
			name:       "both latest, differing digests",
			runningTag: "latest", runningDigest: "sha256:dead",
			latestTag: "latest", latestDigest: "sha256:beef",
			want: false,
		},
		{
			name:       "both latest, no digests -> cannot confirm",
			runningTag: "latest", runningDigest: "",
			latestTag: "latest", latestDigest: "",
			want: false,
		},
		{
			name:       "running latest vs explicit latest tag, no digest -> not up to date",
			runningTag: "latest", runningDigest: "",
			latestTag: "v1.0.0", latestDigest: "",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsVersionUpToDate(tt.runningTag, tt.runningDigest, tt.latestTag, tt.latestDigest)
			if got != tt.want {
				t.Errorf("IsVersionUpToDate(%q,%q,%q,%q) = %v, want %v",
					tt.runningTag, tt.runningDigest, tt.latestTag, tt.latestDigest, got, tt.want)
			}
		})
	}
}

func TestResolveContainerVersion(t *testing.T) {
	tests := []struct {
		name     string
		imageTag string
		labels   map[string]string
		want     string
	}{
		{
			name:     "explicit tag wins over labels",
			imageTag: "v1.2.3",
			labels:   map[string]string{"org.opencontainers.image.version": "9.9.9"},
			want:     "v1.2.3",
		},
		{
			name:     "latest falls back to OCI version label",
			imageTag: "latest",
			labels:   map[string]string{"org.opencontainers.image.version": "2.4.6"},
			want:     "2.4.6",
		},
		{
			name:     "label priority order: OCI before label-schema before version",
			imageTag: "latest",
			labels: map[string]string{
				"org.label-schema.version": "schema",
				"version":                  "plain",
			},
			want: "schema",
		},
		{
			name:     "latest with no useful labels stays latest",
			imageTag: "latest",
			labels:   map[string]string{"unrelated": "x"},
			want:     "latest",
		},
		{
			name:     "latest with nil labels stays latest",
			imageTag: "latest",
			labels:   nil,
			want:     "latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveContainerVersion(tt.imageTag, tt.labels); got != tt.want {
				t.Errorf("ResolveContainerVersion(%q, %v) = %q, want %q", tt.imageTag, tt.labels, got, tt.want)
			}
		})
	}
}
