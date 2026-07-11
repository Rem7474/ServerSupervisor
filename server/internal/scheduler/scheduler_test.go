package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// fakeDB is a minimal in-memory implementation of the scheduler's DB port.
// It records calls instead of touching a real database, so tests exercise the
// cron registration mechanics (Add/Remove/Update/NextRun/Start) without ever
// needing a job to actually fire (which would require a real dispatcher).
type fakeDB struct {
	mu    sync.Mutex
	tasks []models.ScheduledTask
	runs  []runCall
}

type runCall struct {
	id, status string
}

func (f *fakeDB) GetAllScheduledTasks(_ context.Context) ([]models.ScheduledTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.ScheduledTask{}, f.tasks...), nil
}

func (f *fakeDB) UpdateScheduledTaskRun(_ context.Context, id, status string, _, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, runCall{id, status})
	return nil
}

func (f *fakeDB) LinkCommandToScheduledTask(_ context.Context, _, _ string) error {
	return nil
}

// newStartedTestScheduler starts the scheduler with an empty DB (no
// pre-existing tasks) before returning it, mirroring production: main.go
// always calls Start() once at boot, and Add/Update/Remove are only ever
// invoked afterward, from the HTTP handlers. robfig/cron only computes an
// entry's Next time once its internal loop is actually running (Start()),
// so calling Add on a never-started scheduler leaves NextRun at zero — this
// helper avoids that trap in every test below.
func newStartedTestScheduler(t *testing.T, db *fakeDB) *TaskScheduler {
	t.Helper()
	s := New(db, dispatch.New(nil))
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	t.Cleanup(func() {
		s.Stop()
		cancel()
	})
	return s
}

// safeCron is a valid 5-field cron expression (Jan 1st, midnight) chosen so it
// cannot plausibly fire during a test run — these tests exercise the
// registration mechanics (Add/Remove/Update/NextRun), never an actual firing,
// which would call the scheduler's real dispatcher with a nil DB below.
const safeCron = "0 0 1 1 *"

// safeCronAlt is a second never-fires-during-tests expression, distinct from
// safeCron, used to prove Update actually replaced the cron entry.
const safeCronAlt = "0 0 1 6 *"

func TestAdd_RegistersJobAndComputesNextRun(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	task := models.ScheduledTask{ID: "t1", Name: "test", CronExpression: safeCron, Enabled: true}

	if err := s.Add(task); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	next := s.NextRun("t1")
	if next.IsZero() {
		t.Error("NextRun returned zero time for a just-registered task")
	}
	if !next.After(time.Now()) {
		t.Errorf("NextRun = %v, want a time in the future", next)
	}
}

func TestAdd_InvalidCronExpressionReturnsError(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	task := models.ScheduledTask{ID: "t1", Name: "bad", CronExpression: "not a cron expression"}

	if err := s.Add(task); err == nil {
		t.Error("expected an error for an invalid cron expression, got nil")
	}
}

func TestNextRun_UnknownTaskReturnsZero(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	if next := s.NextRun("does-not-exist"); !next.IsZero() {
		t.Errorf("NextRun for an unregistered task = %v, want zero time", next)
	}
}

func TestRemove_UnregistersJob(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	task := models.ScheduledTask{ID: "t1", CronExpression: safeCron, Enabled: true}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if next := s.NextRun("t1"); next.IsZero() {
		t.Fatal("precondition failed: NextRun was zero right after Add")
	}

	s.Remove("t1")

	if next := s.NextRun("t1"); !next.IsZero() {
		t.Errorf("NextRun after Remove = %v, want zero time", next)
	}
}

func TestUpdate_ReReregistersWithNewSchedule(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	task := models.ScheduledTask{ID: "t1", CronExpression: safeCron, Enabled: true}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	firstNext := s.NextRun("t1")

	// Updating to a different schedule should change the computed next-run
	// time, proving the old entry was actually replaced rather than kept.
	task.CronExpression = safeCronAlt
	if err := s.Update(task); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	secondNext := s.NextRun("t1")

	if secondNext.IsZero() {
		t.Fatal("NextRun after Update returned zero time")
	}
	if secondNext.Equal(firstNext) {
		t.Error("NextRun did not change after Update — job was not re-registered")
	}
}

func TestUpdate_DisabledTaskIsRemovedNotReAdded(t *testing.T) {
	s := newStartedTestScheduler(t, &fakeDB{})
	task := models.ScheduledTask{ID: "t1", CronExpression: safeCron, Enabled: true}
	if err := s.Add(task); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	task.Enabled = false
	if err := s.Update(task); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if next := s.NextRun("t1"); !next.IsZero() {
		t.Errorf("NextRun for a disabled task = %v, want zero time", next)
	}
}

func TestStart_LoadsAndRegistersExistingTasks(t *testing.T) {
	db := &fakeDB{tasks: []models.ScheduledTask{
		{ID: "t1", CronExpression: safeCron, Enabled: true},
		{ID: "t2", CronExpression: safeCron, Enabled: true},
	}}
	s := New(db, dispatch.New(nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Stop()

	for _, id := range []string{"t1", "t2"} {
		if next := s.NextRun(id); next.IsZero() {
			t.Errorf("task %q was not registered by Start: NextRun returned zero time", id)
		}
	}
}

func TestStart_SkipsTaskWithInvalidCronButRegistersOthers(t *testing.T) {
	db := &fakeDB{tasks: []models.ScheduledTask{
		{ID: "bad", CronExpression: "not a cron expression", Enabled: true},
		{ID: "good", CronExpression: safeCron, Enabled: true},
	}}
	s := New(db, dispatch.New(nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Stop()

	if next := s.NextRun("bad"); !next.IsZero() {
		t.Errorf("invalid-cron task should not be registered, got NextRun = %v", next)
	}
	if next := s.NextRun("good"); next.IsZero() {
		t.Error("valid task after a bad one was not registered — one bad entry should not block the rest")
	}
}
