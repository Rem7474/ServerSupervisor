package docker

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	all []models.DockerContainer
}

func (f *fakeRepo) GetDockerContainers(context.Context, string) ([]models.DockerContainer, error) {
	return nil, nil
}
func (f *fakeRepo) GetAllDockerContainers(context.Context) ([]models.DockerContainer, error) {
	return f.all, nil
}
func (f *fakeRepo) GetAllComposeProjects(context.Context) ([]models.ComposeProject, error) {
	return nil, nil
}
func (f *fakeRepo) GetComposeProjectsByHost(context.Context, string) ([]models.ComposeProject, error) {
	return nil, nil
}

type fakeDispatcher struct{ req dispatch.Request }

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.req = req
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd"}}, nil
}

// TestIsValidWorkingDir guards the path-traversal check. filepath.IsAbs is
// OS-specific, so absolute-path cases are only asserted on Linux (the agent target);
// the relative-path rejection is portable and is the core of the guard.
func TestIsValidWorkingDir(t *testing.T) {
	portable := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"relative/path", false},
		{"./app", false},
		{"../etc", false},
		{"a/../b", false},
	}
	for _, c := range portable {
		if got := isValidWorkingDir(c.in); got != c.want {
			t.Errorf("isValidWorkingDir(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	if runtime.GOOS == "linux" {
		for _, in := range []string{"/opt/app", "/srv/stacks/web"} {
			if !isValidWorkingDir(in) {
				t.Errorf("isValidWorkingDir(%q) = false, want true (linux)", in)
			}
		}
	}
}

func TestSendCommand_RejectsRelativeWorkingDir(t *testing.T) {
	disp := &fakeDispatcher{}
	_, err := NewService(&fakeRepo{}, disp).SendCommand(context.Background(),
		models.DockerCommandRequest{HostID: "h1", ContainerName: "web", Action: "compose_up", WorkingDir: "relative/dir"},
		"alice", "1.2.3.4")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("relative working_dir should be apperr 400, got %v", err)
	}
	if disp.req.Module != "" {
		t.Error("must not dispatch when validation fails")
	}
}

func TestSendCommand_Dispatches(t *testing.T) {
	disp := &fakeDispatcher{}
	id, err := NewService(&fakeRepo{}, disp).SendCommand(context.Background(),
		models.DockerCommandRequest{HostID: "h1", ContainerName: "web", Action: "restart"},
		"alice", "1.2.3.4")
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if id != "cmd" || disp.req.Module != "docker" || disp.req.Action != "restart" || disp.req.Target != "web" {
		t.Errorf("unexpected dispatch: id=%q req=%+v", id, disp.req)
	}
}

func TestAllContainers_Paginates(t *testing.T) {
	all := make([]models.DockerContainer, 5)
	svc := NewService(&fakeRepo{all: all}, &fakeDispatcher{})

	page, total, _ := svc.AllContainers(context.Background(), 2, 1)
	if total != 5 || len(page) != 2 {
		t.Errorf("page=%d total=%d, want 2/5", len(page), total)
	}
	// Offset past the end yields an empty (non-nil) page with the real total.
	page, total, _ = svc.AllContainers(context.Background(), 2, 10)
	if total != 5 || page == nil || len(page) != 0 {
		t.Errorf("offset-past-end: page=%v total=%d, want empty/5", page, total)
	}
}
