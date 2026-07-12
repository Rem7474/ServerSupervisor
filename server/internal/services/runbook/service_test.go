package runbook

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	runbooks   map[string]*models.Runbook
	executions map[string]*models.RunbookExecution
	commands   map[string]*models.RemoteCommand
	validHosts map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		runbooks:   map[string]*models.Runbook{},
		executions: map[string]*models.RunbookExecution{},
		commands:   map[string]*models.RemoteCommand{},
		validHosts: map[string]bool{"host-1": true, "host-2": true},
	}
}

func stepsFor(id string, steps []models.RunbookStepCreate) []models.RunbookStep {
	out := make([]models.RunbookStep, 0, len(steps))
	for i, s := range steps {
		out = append(out, models.RunbookStep{
			ID: fmt.Sprintf("%s-step-%d", id, i), RunbookID: id, Position: i,
			HostID: s.HostID, Module: s.Module, Action: s.Action, Target: s.Target,
			Payload: s.Payload, ContinueOnFailure: s.ContinueOnFailure,
		})
	}
	return out
}

func (f *fakeRepo) CreateRunbook(_ context.Context, name, description string, steps []models.RunbookStepCreate) (*models.Runbook, error) {
	id := fmt.Sprintf("rb-%d", len(f.runbooks)+1)
	rb := &models.Runbook{ID: id, Name: name, Description: description, Steps: stepsFor(id, steps)}
	f.runbooks[id] = rb
	return rb, nil
}

func (f *fakeRepo) GetRunbook(_ context.Context, id string) (*models.Runbook, error) {
	rb, ok := f.runbooks[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *rb
	return &cp, nil
}

func (f *fakeRepo) ListRunbooks(context.Context) ([]models.Runbook, error) {
	out := []models.Runbook{}
	for _, rb := range f.runbooks {
		out = append(out, *rb)
	}
	return out, nil
}

func (f *fakeRepo) UpdateRunbook(_ context.Context, id string, name, description *string, steps *[]models.RunbookStepCreate) error {
	rb, ok := f.runbooks[id]
	if !ok {
		return sql.ErrNoRows
	}
	if name != nil {
		rb.Name = *name
	}
	if description != nil {
		rb.Description = *description
	}
	if steps != nil {
		rb.Steps = stepsFor(id, *steps)
	}
	return nil
}

func (f *fakeRepo) DeleteRunbook(_ context.Context, id string) error {
	delete(f.runbooks, id)
	return nil
}

func (f *fakeRepo) HostExists(_ context.Context, id string) (bool, error) {
	return f.validHosts[id], nil
}

func (f *fakeRepo) CreateRunbookExecution(_ context.Context, runbookID, triggeredBy string) (*models.RunbookExecution, error) {
	id := fmt.Sprintf("exec-%d", len(f.executions)+1)
	exec := &models.RunbookExecution{ID: id, RunbookID: runbookID, Status: "running", TriggeredBy: triggeredBy}
	f.executions[id] = exec
	return exec, nil
}

func (f *fakeRepo) GetRunbookExecution(_ context.Context, id string) (*models.RunbookExecution, error) {
	exec, ok := f.executions[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *exec
	return &cp, nil
}

func (f *fakeRepo) ListRunbookExecutions(_ context.Context, runbookID string, _ int) ([]models.RunbookExecution, error) {
	out := []models.RunbookExecution{}
	for _, e := range f.executions {
		if e.RunbookID == runbookID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListRunbookExecutionSteps(context.Context, string) ([]models.RunbookExecutionStep, error) {
	return nil, nil
}

func (f *fakeRepo) GetRunbookStepByPosition(_ context.Context, runbookID string, position int) (*models.RunbookStep, error) {
	rb, ok := f.runbooks[runbookID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	for _, s := range rb.Steps {
		if s.Position == position {
			cp := s
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeRepo) AdvanceRunbookExecution(_ context.Context, executionID string, position int) error {
	exec, ok := f.executions[executionID]
	if !ok {
		return sql.ErrNoRows
	}
	exec.CurrentStepPosition = position
	return nil
}

func (f *fakeRepo) FinishRunbookExecution(_ context.Context, executionID, status string) error {
	exec, ok := f.executions[executionID]
	if !ok {
		return sql.ErrNoRows
	}
	exec.Status = status
	return nil
}

func (f *fakeRepo) LinkCommandToRunbookExecution(_ context.Context, commandID, executionID string) error {
	if cmd, ok := f.commands[commandID]; ok {
		eid := executionID
		cmd.RunbookExecutionID = &eid
	}
	return nil
}

func (f *fakeRepo) GetRemoteCommandByID(_ context.Context, id string) (*models.RemoteCommand, error) {
	cmd, ok := f.commands[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *cmd
	return &cp, nil
}

// fakeDispatcher records every dispatched request and registers a fake
// command into the shared fakeRepo, so LinkCommandToRunbookExecution and
// GetRemoteCommandByID see it exactly like the real dispatcher+DB would.
type fakeDispatcher struct {
	repo     *fakeRepo
	requests []dispatch.Request
	failNext bool
}

func (d *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	d.requests = append(d.requests, req)
	if d.failNext {
		d.failNext = false
		return nil, fmt.Errorf("dispatch failed")
	}
	id := fmt.Sprintf("cmd-%d", len(d.requests))
	cmd := &models.RemoteCommand{ID: id, HostID: req.HostID, Module: req.Module, Action: req.Action, Target: req.Target, Status: "pending"}
	d.repo.commands[id] = cmd
	return &dispatch.Result{Command: cmd}, nil
}

func twoStepRunbook(t *testing.T, svc *Service, secondContinueOnFailure bool) *models.Runbook {
	t.Helper()
	rb, err := svc.Create(context.Background(), models.RunbookCreate{
		Name: "Restart pipeline",
		Steps: []models.RunbookStepCreate{
			{HostID: "host-1", Module: "docker", Action: "stop", Target: "app"},
			{HostID: "host-2", Module: "systemd", Action: "restart", Target: "nginx", ContinueOnFailure: secondContinueOnFailure},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return rb
}

func TestCreate_ValidatesSteps(t *testing.T) {
	cases := []struct {
		name  string
		steps []models.RunbookStepCreate
	}{
		{"no steps", nil},
		{"missing host", []models.RunbookStepCreate{{Module: "docker", Action: "stop"}}},
		{"unknown host", []models.RunbookStepCreate{{HostID: "nope", Module: "docker", Action: "stop"}}},
		{"unknown module", []models.RunbookStepCreate{{HostID: "host-1", Module: "ssh", Action: "run"}}},
		{"invalid action for module", []models.RunbookStepCreate{{HostID: "host-1", Module: "docker", Action: "delete"}}},
		{"missing target for a target-requiring module", []models.RunbookStepCreate{{HostID: "host-1", Module: "systemd", Action: "restart"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := NewService(repo, &fakeDispatcher{repo: repo})
			_, err := svc.Create(context.Background(), models.RunbookCreate{Name: "x", Steps: tc.steps})
			if err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestRun_DispatchesFirstStepOnly(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	rb := twoStepRunbook(t, svc, false)

	exec, err := svc.Run(context.Background(), rb.ID, "alice")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(disp.requests) != 1 {
		t.Fatalf("dispatched %d requests, want 1 (only the first step)", len(disp.requests))
	}
	if disp.requests[0].HostID != "host-1" || disp.requests[0].Action != "stop" {
		t.Errorf("dispatched wrong step: %+v", disp.requests[0])
	}
	if exec.Status != "running" {
		t.Errorf("execution status = %q, want running", exec.Status)
	}
}

func TestNotifyComplete_AdvancesToNextStepOnSuccess(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	rb := twoStepRunbook(t, svc, false)

	exec, err := svc.Run(context.Background(), rb.ID, "alice")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	firstCmdID := disp.requests0ID(t)

	svc.NotifyComplete(context.Background(), firstCmdID, "completed")

	if len(disp.requests) != 2 {
		t.Fatalf("dispatched %d requests, want 2 (step 2 should have been dispatched)", len(disp.requests))
	}
	if disp.requests[1].HostID != "host-2" || disp.requests[1].Action != "restart" {
		t.Errorf("dispatched wrong second step: %+v", disp.requests[1])
	}
	got, _ := repo.GetRunbookExecution(context.Background(), exec.ID)
	if got.Status != "running" {
		t.Errorf("execution status = %q, want still running", got.Status)
	}
	if got.CurrentStepPosition != 1 {
		t.Errorf("current_step_position = %d, want 1", got.CurrentStepPosition)
	}
}

func TestNotifyComplete_FinishesCompletedWhenNoMoreSteps(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	rb := twoStepRunbook(t, svc, false)

	exec, _ := svc.Run(context.Background(), rb.ID, "alice")
	firstCmdID := disp.requests0ID(t)
	svc.NotifyComplete(context.Background(), firstCmdID, "completed")
	secondCmdID := disp.requestsLastID(t)
	svc.NotifyComplete(context.Background(), secondCmdID, "completed")

	got, _ := repo.GetRunbookExecution(context.Background(), exec.ID)
	if got.Status != "completed" {
		t.Errorf("execution status = %q, want completed", got.Status)
	}
	if len(disp.requests) != 2 {
		t.Errorf("dispatched %d requests, want exactly 2 (no phantom third step)", len(disp.requests))
	}
}

func TestNotifyComplete_StopsOnFailureByDefault(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	rb := twoStepRunbook(t, svc, false) // continue_on_failure=false on step 2, but we fail step 1

	exec, _ := svc.Run(context.Background(), rb.ID, "alice")
	firstCmdID := disp.requests0ID(t)

	svc.NotifyComplete(context.Background(), firstCmdID, "failed")

	got, _ := repo.GetRunbookExecution(context.Background(), exec.ID)
	if got.Status != "failed" {
		t.Errorf("execution status = %q, want failed", got.Status)
	}
	if len(disp.requests) != 1 {
		t.Errorf("dispatched %d requests, want 1 (step 2 must not run after an unflagged failure)", len(disp.requests))
	}
}

func TestNotifyComplete_ContinuesOnFailureWhenStepFlagged(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	// Flag STEP 1 (the one we're about to fail) to continue on failure by
	// building a runbook where step 0 carries the flag.
	rbResp, err := svc.Create(context.Background(), models.RunbookCreate{
		Name: "Tolerant pipeline",
		Steps: []models.RunbookStepCreate{
			{HostID: "host-1", Module: "docker", Action: "stop", Target: "app", ContinueOnFailure: true},
			{HostID: "host-2", Module: "systemd", Action: "restart", Target: "nginx"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc.Run(context.Background(), rbResp.ID, "alice") //nolint:errcheck
	firstCmdID := disp.requests0ID(t)

	svc.NotifyComplete(context.Background(), firstCmdID, "failed")

	if len(disp.requests) != 2 {
		t.Fatalf("dispatched %d requests, want 2 (step 2 should still run: step 1 tolerates failure)", len(disp.requests))
	}
}

func TestNotifyComplete_IgnoresNonRunbookCommand(t *testing.T) {
	repo := newFakeRepo()
	repo.commands["standalone-cmd"] = &models.RemoteCommand{ID: "standalone-cmd", HostID: "host-1", Status: "completed"}
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)

	svc.NotifyComplete(context.Background(), "standalone-cmd", "completed")

	if len(disp.requests) != 0 {
		t.Errorf("a command with no runbook_execution_id must never trigger a dispatch, got %d", len(disp.requests))
	}
}

func TestNotifyComplete_IgnoresAlreadyTerminalExecution(t *testing.T) {
	repo := newFakeRepo()
	disp := &fakeDispatcher{repo: repo}
	svc := NewService(repo, disp)
	rb := twoStepRunbook(t, svc, false)

	exec, _ := svc.Run(context.Background(), rb.ID, "alice")
	firstCmdID := disp.requests0ID(t)
	svc.NotifyComplete(context.Background(), firstCmdID, "failed") // terminates the execution as "failed"

	// A late/duplicate completion event for the same command must not re-trigger anything.
	svc.NotifyComplete(context.Background(), firstCmdID, "failed")

	got, _ := repo.GetRunbookExecution(context.Background(), exec.ID)
	if got.Status != "failed" {
		t.Errorf("execution status = %q, want failed", got.Status)
	}
	if len(disp.requests) != 1 {
		t.Errorf("dispatched %d requests, want 1 — the duplicate event must be a no-op", len(disp.requests))
	}
}

// requests0ID/requestsLastID return the command ID that dispatchStep
// generated for the first/last recorded request, by re-deriving fakeDispatcher's
// own "cmd-<n>" naming scheme (kept in one place to avoid repeating it in every test).
func (d *fakeDispatcher) requests0ID(t *testing.T) string {
	t.Helper()
	if len(d.requests) < 1 {
		t.Fatal("expected at least 1 dispatched request")
	}
	return "cmd-1"
}

func (d *fakeDispatcher) requestsLastID(t *testing.T) string {
	t.Helper()
	if len(d.requests) < 1 {
		t.Fatal("expected at least 1 dispatched request")
	}
	return fmt.Sprintf("cmd-%d", len(d.requests))
}
