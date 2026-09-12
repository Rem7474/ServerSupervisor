package runbook

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// wantCode asserts an error is an *apperr.Error carrying the expected catalog
// key. The key is what respondError needs to render the message in the
// caller's language, so it is the part worth pinning — not the wording.
func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error carrying %s, got nil", code)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperr.Error, got %T: %v", err, err)
	}
	if ae.I18nKey != code {
		t.Fatalf("i18n key = %q, want %q (err: %v)", ae.I18nKey, code, err)
	}
	if rendered := apperr.GetMessage(code, "fr", ae.Params); rendered == "error: "+code {
		t.Errorf("catalog has no FR entry for %q", code)
	}
}

func newSvc(repo *fakeRepo) *Service {
	return NewService(repo, &fakeDispatcher{repo: repo})
}

func validSteps() []models.RunbookStepCreate {
	return []models.RunbookStepCreate{{HostID: "host-1", Module: "docker", Action: "restart", Target: "web"}}
}

func seedRunbook(t *testing.T, repo *fakeRepo, steps []models.RunbookStepCreate) *models.Runbook {
	t.Helper()
	rb, err := newSvc(repo).Create(context.Background(), models.RunbookCreate{Name: "rb", Steps: steps})
	if err != nil {
		t.Fatalf("seed runbook: %v", err)
	}
	return rb
}

func TestCreate_RejectsABlankName(t *testing.T) {
	for _, name := range []string{"", "   "} {
		err := func() error {
			_, err := newSvc(newFakeRepo()).Create(context.Background(), models.RunbookCreate{Name: name, Steps: validSteps()})
			return err
		}()
		wantCode(t, err, apperr.CodeRunbookNameRequired)
	}
}

func TestCreate_ReportsARepositoryFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.failOn = "CreateRunbook"

	_, err := newSvc(repo).Create(context.Background(), models.RunbookCreate{Name: "rb", Steps: validSteps()})

	wantCode(t, err, apperr.CodeRunbookCreateFailed)
}

func TestValidateSteps_ReportsAHostLookupFailure(t *testing.T) {
	// A repo error while checking a step's host is not "host not found" — the
	// step may well be valid, we just couldn't tell.
	repo := newFakeRepo()
	repo.failOn = "HostExists"

	_, err := newSvc(repo).Create(context.Background(), models.RunbookCreate{Name: "rb", Steps: validSteps()})

	wantCode(t, err, apperr.CodeRunbookStepsInvalid)
}

func TestUpdate_UnknownRunbook(t *testing.T) {
	err := newSvc(newFakeRepo()).Update(context.Background(), "nope", models.RunbookUpdate{})
	wantCode(t, err, apperr.CodeRunbookNotFound)
}

func TestUpdate_RejectsABlankName(t *testing.T) {
	repo := newFakeRepo()
	rb := seedRunbook(t, repo, validSteps())
	blank := "  "

	err := newSvc(repo).Update(context.Background(), rb.ID, models.RunbookUpdate{Name: &blank})

	wantCode(t, err, apperr.CodeRunbookNameRequired)
}

func TestUpdate_RevalidatesStepsWhenTheyChange(t *testing.T) {
	repo := newFakeRepo()
	rb := seedRunbook(t, repo, validSteps())
	bad := []models.RunbookStepCreate{{HostID: "unknown-host", Module: "docker", Action: "restart"}}

	err := newSvc(repo).Update(context.Background(), rb.ID, models.RunbookUpdate{Steps: &bad})

	wantCode(t, err, apperr.CodeRunbookStepHostNotFound)
}

func TestUpdate_ReportsARepositoryFailure(t *testing.T) {
	repo := newFakeRepo()
	rb := seedRunbook(t, repo, validSteps())
	repo.failOn = "UpdateRunbook"
	name := "renamed"

	err := newSvc(repo).Update(context.Background(), rb.ID, models.RunbookUpdate{Name: &name})

	wantCode(t, err, apperr.CodeRunbookUpdateFailed)
}

func TestDelete_UnknownRunbook(t *testing.T) {
	err := newSvc(newFakeRepo()).Delete(context.Background(), "nope")
	wantCode(t, err, apperr.CodeRunbookNotFound)
}

func TestRun_UnknownRunbook(t *testing.T) {
	_, err := newSvc(newFakeRepo()).Run(context.Background(), "nope", "alice")
	wantCode(t, err, apperr.CodeRunbookNotFound)
}

func TestRun_RunbookWithNoSteps(t *testing.T) {
	// Create() rejects an empty step list, so an existing runbook can only get
	// here by having its steps removed underneath us — Run must still refuse
	// rather than create an execution that can never advance.
	repo := newFakeRepo()
	rb := seedRunbook(t, repo, validSteps())
	repo.runbooks[rb.ID].Steps = nil

	_, err := newSvc(repo).Run(context.Background(), rb.ID, "alice")

	wantCode(t, err, apperr.CodeRunbookNoSteps)
}

func TestRun_ReportsAnExecutionCreationFailure(t *testing.T) {
	repo := newFakeRepo()
	rb := seedRunbook(t, repo, validSteps())
	repo.failOn = "CreateRunbookExecution"

	_, err := newSvc(repo).Run(context.Background(), rb.ID, "alice")

	wantCode(t, err, apperr.CodeRunbookRunFailed)
	if len(repo.commands) != 0 {
		t.Errorf("no step should be dispatched when the execution was never created, got %d", len(repo.commands))
	}
}

func TestGetExecution_UnknownExecution(t *testing.T) {
	_, err := newSvc(newFakeRepo()).GetExecution(context.Background(), "nope")
	wantCode(t, err, apperr.CodeRunbookExecutionNotFound)
}
