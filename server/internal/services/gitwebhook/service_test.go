package gitwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// ===== pure helpers =====

func TestVerifyWebhookSignature_GitHub(t *testing.T) {
	secret := "s3cr3t"
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	h := http.Header{}
	h.Set("X-Hub-Signature-256", sig)
	if !verifyWebhookSignature("github", secret, body, h) {
		t.Error("valid signature rejected")
	}
	bad := http.Header{}
	bad.Set("X-Hub-Signature-256", "sha256=deadbeef")
	if verifyWebhookSignature("github", secret, body, bad) {
		t.Error("invalid signature accepted")
	}
}

func TestVerifyWebhookSignature_GitLabToken(t *testing.T) {
	h := http.Header{}
	h.Set("X-Gitlab-Token", "tok")
	if !verifyWebhookSignature("gitlab", "tok", nil, h) {
		t.Error("matching gitlab token rejected")
	}
	if verifyWebhookSignature("gitlab", "other", nil, h) {
		t.Error("mismatched gitlab token accepted")
	}
}

func TestParseGitHubPayload(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main","after":"abc123","head_commit":{"id":"abc123","message":"fix: thing\nbody"},"repository":{"full_name":"acme/app"},"pusher":{"name":"alice"}}`)
	p, err := parseGitHubPayload(body, "push")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.RepoName != "acme/app" || p.Branch != "main" || p.CommitSHA != "abc123" || p.CommitMessage != "fix: thing" || p.Pusher != "alice" {
		t.Errorf("unexpected parse: %+v", p)
	}
}

func TestHelpers(t *testing.T) {
	if refToBranch("refs/heads/dev") != "dev" {
		t.Error("refToBranch heads")
	}
	if refToBranch("refs/tags/v1") != "v1" {
		t.Error("refToBranch tags")
	}
	if normalizeEvent("Push") != "push" || normalizeEvent("create") != "tag" {
		t.Error("normalizeEvent")
	}
	if !matchFilter("", "anything") || !matchFilter("main", "main") || !matchFilter("feat/*", "feat/x") {
		t.Error("matchFilter should match")
	}
	if matchFilter("main", "dev") {
		t.Error("matchFilter should not match")
	}
}

// ===== service with a fake repo/dispatcher =====

type fakeRepo struct {
	receiveWH *models.GitWebhook
	running   bool
	execID    string
}

func (fakeRepo) ListGitWebhooks(context.Context) ([]models.GitWebhook, error) { return nil, nil }
func (fakeRepo) CreateGitWebhook(_ context.Context, w models.GitWebhook) (*models.GitWebhook, error) {
	return &w, nil
}
func (fakeRepo) GetGitWebhookByID(context.Context, string) (*models.GitWebhook, error) {
	return nil, nil
}
func (f fakeRepo) GetGitWebhookForReceive(context.Context, string) (*models.GitWebhook, error) {
	return f.receiveWH, nil
}
func (fakeRepo) UpdateGitWebhook(context.Context, string, models.GitWebhook) error { return nil }
func (fakeRepo) DeleteGitWebhook(context.Context, string) error                    { return nil }
func (fakeRepo) RegenerateWebhookSecret(context.Context, string) (string, error)   { return "new", nil }
func (fakeRepo) UpdateGitWebhookLastTriggered(context.Context, string) error       { return nil }
func (f fakeRepo) CreateWebhookExecution(_ context.Context, e models.GitWebhookExecution) (*models.GitWebhookExecution, error) {
	e.ID = f.execID
	return &e, nil
}
func (fakeRepo) UpdateWebhookExecutionCommandID(context.Context, string, string) error { return nil }
func (fakeRepo) UpdateWebhookExecutionStatus(context.Context, string, string, *time.Time) error {
	return nil
}
func (fakeRepo) UpdateWebhookExecutionByCommandID(context.Context, string, string) (string, bool, bool, []string, error) {
	return "", false, false, nil, nil
}
func (f fakeRepo) GetRunningExecutionForWebhook(context.Context, string) (bool, error) {
	return f.running, nil
}
func (fakeRepo) ListWebhookExecutions(context.Context, string, int) ([]models.GitWebhookExecution, error) {
	return nil, nil
}

type fakeDispatcher struct{ called bool }

func (f *fakeDispatcher) Create(context.Context, dispatch.Request) (*dispatch.Result, error) {
	f.called = true
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-1"}}, nil
}

func signed(secret string, body []byte) http.Header {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	h.Set("X-GitHub-Event", "push")
	return h
}

func TestCreate_Validation(t *testing.T) {
	svc := NewService(fakeRepo{}, nil, &fakeDispatcher{}, nil)
	_, err := svc.Create(context.Background(), models.GitWebhookRequest{Provider: "github", HostID: "h", CustomTaskID: "t"})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("missing name should be 400, got %v", err)
	}
	_, err = svc.Create(context.Background(), models.GitWebhookRequest{Name: "x", Provider: "svn", HostID: "h", CustomTaskID: "t"})
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("bad provider should be 400, got %v", err)
	}
}

func TestReceive_InvalidSignature(t *testing.T) {
	wh := &models.GitWebhook{Enabled: true, Provider: "github", Secret: "s"}
	svc := NewService(fakeRepo{receiveWH: wh}, nil, &fakeDispatcher{}, nil)
	bad := http.Header{}
	bad.Set("X-Hub-Signature-256", "sha256=00")
	_, err := svc.Receive(context.Background(), "id", []byte(`{}`), bad, "1.2.3.4")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 401 {
		t.Fatalf("bad signature should be 401, got %v", err)
	}
}

func TestReceive_RepoFilterSkips(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main","repository":{"full_name":"acme/app"}}`)
	wh := &models.GitWebhook{Enabled: true, Provider: "github", Secret: "s", EventFilter: "push", RepoFilter: "other/repo"}
	disp := &fakeDispatcher{}
	svc := NewService(fakeRepo{receiveWH: wh}, nil, disp, nil)
	res, err := svc.Receive(context.Background(), "id", body, signed("s", body), "1.2.3.4")
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if res.Status != "skipped" || res.Reason != "repo_filter" {
		t.Errorf("expected repo_filter skip, got %+v", res)
	}
	if disp.called {
		t.Error("must not dispatch when repo filter excludes the push")
	}
}

func TestReceive_Dispatches(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/main","after":"sha","repository":{"full_name":"acme/app"}}`)
	wh := &models.GitWebhook{Enabled: true, Provider: "github", Secret: "s", EventFilter: "push", HostID: "h1", CustomTaskID: "t1", Name: "wh"}
	disp := &fakeDispatcher{}
	svc := NewService(fakeRepo{receiveWH: wh, execID: "exec-1"}, nil, disp, nil)
	res, err := svc.Receive(context.Background(), "id", body, signed("s", body), "1.2.3.4")
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if res.Status != "dispatched" || res.CommandID != "cmd-1" || res.ExecutionID != "exec-1" {
		t.Errorf("unexpected dispatch result: %+v", res)
	}
	if !disp.called {
		t.Error("expected a command dispatch")
	}
}
