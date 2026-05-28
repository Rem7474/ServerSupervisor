package background

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

// Job is a named background task that runs until its context is cancelled.
type Job struct {
	Name string
	Run  func(ctx context.Context)
}

// Runner manages a set of background jobs.
// Each job runs in its own goroutine with panic recovery.
// Call Start() once after registering jobs; Stop() signals cancellation and
// waits for all goroutines to exit before returning.
type Runner struct {
	jobs   []Job
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates an idle Runner ready to accept jobs.
func New() *Runner {
	return &Runner{}
}

// Add registers a job. Must be called before Start.
func (r *Runner) Add(job Job) {
	r.jobs = append(r.jobs, job)
}

// Start launches all registered jobs concurrently.
// The provided parent ctx is the root cancellation signal (typically the one
// owned by cmd/server/main.go and wired to SIGINT/SIGTERM). Cancelling parent
// or calling Stop both terminate every job.
func (r *Runner) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	for _, job := range r.jobs {
		r.wg.Add(1)
		go r.run(ctx, job)
	}
	slog.Info("background jobs started", slog.Int("count", len(r.jobs)))
}

// Stop cancels the shared context and waits for all jobs to return.
func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	slog.Info("background jobs stopped")
}

func (r *Runner) run(ctx context.Context, job Job) {
	defer r.wg.Done()
	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "background job panicked",
				slog.String("job", job.Name),
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())))
		}
	}()
	job.Run(ctx)
}
