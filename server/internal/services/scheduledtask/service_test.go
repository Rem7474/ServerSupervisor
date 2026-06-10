package scheduledtask

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	created       *models.ScheduledTask
	updated       *models.ScheduledTask
	getTask       *models.ScheduledTask
	getErr        error
	nextRunSet    bool
	deleted       bool
	linkedCmd     string
	statusUpdated string
}

func (f *fakeRepo) GetGlobalScheduledTasks(context.Context) ([]models.ScheduledTaskWithHost, error) {
	return nil, nil
}
func (f *fakeRepo) GetScheduledTasks(context.Context, string) ([]models.ScheduledTask, error) {
	return nil, nil
}
func (f *fakeRepo) GetScheduledTask(context.Context, string) (*models.ScheduledTask, error) {
	return f.getTask, f.getErr
}
func (f *fakeRepo) CreateScheduledTask(_ context.Context, t models.ScheduledTask) (*models.ScheduledTask, error) {
	cp := t
	cp.ID = "t1"
	f.created = &cp
	return &cp, nil
}
func (f *fakeRepo) UpdateScheduledTask(_ context.Context, _ string, t models.ScheduledTask) error {
	cp := t
	f.updated = &cp
	return nil
}
func (f *fakeRepo) DeleteScheduledTask(context.Context, string) error { f.deleted = true; return nil }
func (f *fakeRepo) SetScheduledTaskNextRun(context.Context, string, time.Time) error {
	f.nextRunSet = true
	return nil
}
func (f *fakeRepo) GetHostCustomTasks(context.Context, string) (string, error)    { return "[]", nil }
func (f *fakeRepo) GetHostTasksConfigYAML(context.Context, string) (string, error) { return "", nil }
func (f *fakeRepo) GetScheduledTaskExecutions(context.Context, string, int) ([]models.RemoteCommand, error) {
	return nil, nil
}
func (f *fakeRepo) LinkCommandToScheduledTask(_ context.Context, cmdID, _ string) error {
	f.linkedCmd = cmdID
	return nil
}
func (f *fakeRepo) UpdateScheduledTaskStatus(_ context.Context, _, status string) error {
	f.statusUpdated = status
	return nil
}

type fakeScheduler struct {
	added   *models.ScheduledTask
	updated *models.ScheduledTask
	removed string
	next    time.Time
}

func (f *fakeScheduler) Add(t models.ScheduledTask) error    { cp := t; f.added = &cp; return nil }
func (f *fakeScheduler) Update(t models.ScheduledTask) error { cp := t; f.updated = &cp; return nil }
func (f *fakeScheduler) Remove(id string)                    { f.removed = id }
func (f *fakeScheduler) NextRun(string) time.Time            { return f.next }

type fakeDispatcher struct {
	gotReq dispatch.Request
	result *dispatch.Result
}

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.gotReq = req
	return f.result, nil
}

func req(module string) models.ScheduledTaskRequest {
	return models.ScheduledTaskRequest{Name: "x", Module: module, Action: "run", CronExpression: "0 3 * * *"}
}

func TestCreate_InvalidModule(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeScheduler{}, &fakeDispatcher{})
	_, err := svc.Create(context.Background(), "h1", "alice", req("bogus"))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid module should be apperr 400, got %v", err)
	}
}

func TestCreate_InvalidCron(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeScheduler{}, &fakeDispatcher{})
	r := req("apt")
	r.CronExpression = "not a cron"
	_, err := svc.Create(context.Background(), "h1", "alice", r)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid cron should be apperr 400, got %v", err)
	}
}

func TestCreate_EnabledSchedulesAndDefaultsPayload(t *testing.T) {
	repo := &fakeRepo{}
	sched := &fakeScheduler{next: time.Now().Add(time.Hour)}
	svc := NewService(repo, sched, &fakeDispatcher{})

	r := req("apt")
	r.Enabled = true // payload omitted -> defaults to "{}"
	created, err := svc.Create(context.Background(), "h1", "alice", r)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.created.Payload != "{}" {
		t.Errorf("payload should default to {}, got %q", repo.created.Payload)
	}
	if repo.created.CreatedBy != "alice" {
		t.Errorf("created_by should be the caller, got %q", repo.created.CreatedBy)
	}
	if sched.added == nil {
		t.Error("an enabled task must be registered with the scheduler")
	}
	if created.NextRunAt == nil {
		t.Error("next_run_at should be populated from the scheduler")
	}
}

func TestCreate_DisabledDoesNotSchedule(t *testing.T) {
	sched := &fakeScheduler{}
	svc := NewService(&fakeRepo{}, sched, &fakeDispatcher{})
	r := req("apt")
	r.Enabled = false
	if _, err := svc.Create(context.Background(), "h1", "alice", r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sched.added != nil {
		t.Error("a disabled task must not be scheduled")
	}
}

func TestGet_NotFound(t *testing.T) {
	svc := NewService(&fakeRepo{getErr: sql.ErrNoRows}, &fakeScheduler{}, &fakeDispatcher{})
	_, err := svc.Get(context.Background(), "missing")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("expected apperr 404, got %v", err)
	}
}

func TestDelete_UnschedulesThenDeletes(t *testing.T) {
	repo := &fakeRepo{getTask: &models.ScheduledTask{ID: "t1"}}
	sched := &fakeScheduler{}
	svc := NewService(repo, sched, &fakeDispatcher{})
	if err := svc.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if sched.removed != "t1" {
		t.Error("Delete must unregister the task from the scheduler")
	}
	if !repo.deleted {
		t.Error("Delete must remove the task from the repository")
	}
}

func TestRun_DispatchesLinksAndMarksPending(t *testing.T) {
	repo := &fakeRepo{}
	disp := &fakeDispatcher{result: &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-9"}}}
	svc := NewService(repo, &fakeScheduler{}, disp)

	task := models.ScheduledTask{ID: "t1", HostID: "h1", Module: "apt", Action: "update", Payload: ""}
	id, err := svc.Run(context.Background(), task, "alice")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if id != "cmd-9" {
		t.Errorf("Run should return the dispatched command id, got %q", id)
	}
	if disp.gotReq.HostID != "h1" || disp.gotReq.Payload != "{}" || disp.gotReq.TriggeredBy != "alice" {
		t.Errorf("dispatch request not built correctly: %+v", disp.gotReq)
	}
	if repo.linkedCmd != "cmd-9" {
		t.Error("Run must link the command to the task")
	}
	if repo.statusUpdated != "pending" {
		t.Errorf("Run must mark the task pending, got %q", repo.statusUpdated)
	}
}
