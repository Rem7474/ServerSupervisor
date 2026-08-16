package releasetracker

import (
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

// The mutable-tag resolution helper this file used to cover
// (shouldResolveDockerTag) moved to internal/services/dockerversions as
// ShouldResolveTag when the poller stopped calling registries itself — see
// dockerversions.TestShouldResolveTag for its cases.

func TestTrackerHasDispatchTarget(t *testing.T) {
	cases := []struct {
		name string
		in   models.ReleaseTracker
		want bool
	}{
		{"custom mode: host + task", models.ReleaseTracker{HostID: "h1", CustomTaskID: "task"}, true},
		{"custom mode: host without task", models.ReleaseTracker{HostID: "h1"}, false},
		{"custom mode: task without host", models.ReleaseTracker{CustomTaskID: "task"}, false},
		{"monitor-only", models.ReleaseTracker{}, false},
		{"compose mode: host + project", models.ReleaseTracker{UpdateAction: "compose", HostID: "h1", ComposeProject: "proj"}, true},
		// A compose tracker deploys the project, not a tasks.yaml command, so a
		// custom_task_id can never stand in for a missing compose_project.
		{"compose mode: task instead of project", models.ReleaseTracker{UpdateAction: "compose", HostID: "h1", CustomTaskID: "task"}, false},
		{"compose mode: project without host", models.ReleaseTracker{UpdateAction: "compose", ComposeProject: "proj"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := trackerHasDispatchTarget(tc.in); got != tc.want {
				t.Errorf("trackerHasDispatchTarget() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasGitLink(t *testing.T) {
	cases := []struct {
		name string
		in   models.ReleaseTracker
		want bool
	}{
		{"docker tracker with a full link", models.ReleaseTracker{TrackerType: "docker", Provider: "github", RepoOwner: "o", RepoName: "r"}, true},
		{"no link at all", models.ReleaseTracker{TrackerType: "docker"}, false},
		{"half a link", models.ReleaseTracker{TrackerType: "docker", Provider: "github", RepoOwner: "o"}, false},
		{"unknown provider", models.ReleaseTracker{TrackerType: "docker", Provider: "svn", RepoOwner: "o", RepoName: "r"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasGitLink(tc.in); got != tc.want {
				t.Errorf("hasGitLink() = %v, want %v", got, tc.want)
			}
		})
	}
}
