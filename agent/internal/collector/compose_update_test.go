package collector

import (
	"reflect"
	"testing"
)

func TestComposeArgs(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		workingDir  string
		rest        []string
		want        []string
	}{
		{
			name:       "with working dir",
			project:    "myapp",
			workingDir: "/srv/myapp",
			rest:       []string{"pull"},
			want:       []string{"compose", "--project-directory", "/srv/myapp", "-p", "myapp", "pull"},
		},
		{
			name:    "no working dir",
			project: "myapp",
			rest:    []string{"up", "-d"},
			want:    []string{"compose", "-p", "myapp", "up", "-d"},
		},
		{
			name:       "with service",
			project:    "myapp",
			workingDir: "/srv/myapp",
			rest:       []string{"up", "-d", "web"},
			want:       []string{"compose", "--project-directory", "/srv/myapp", "-p", "myapp", "up", "-d", "web"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeArgs(tt.project, tt.workingDir, tt.rest...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("composeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitImageRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantRepo string
		wantTag  string
	}{
		{"nginx:1.25", "nginx", "1.25"},
		{"nginx", "nginx", "latest"},
		{"ghcr.io/org/app:v2", "ghcr.io/org/app", "v2"},
		{"registry.example.com:5000/app", "registry.example.com:5000/app", "latest"},
		{"registry.example.com:5000/app:tag", "registry.example.com:5000/app", "tag"},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			repo, tag := splitImageRef(tt.ref)
			if repo != tt.wantRepo || tag != tt.wantTag {
				t.Errorf("splitImageRef(%q) = (%q, %q), want (%q, %q)", tt.ref, repo, tag, tt.wantRepo, tt.wantTag)
			}
		})
	}
}
