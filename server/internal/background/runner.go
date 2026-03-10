package background

import (
	"context"
	"log"
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
func (r *Runner) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	for _, job := range r.jobs {
		r.wg.Add(1)
		go r.run(ctx, job)
	}
	log.Printf("background: started %d jobs", len(r.jobs))
}

// Stop cancels the shared context and waits for all jobs to return.
func (r *Runner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	log.Printf("background: all jobs stopped")
}

func (r *Runner) run(ctx context.Context, job Job) {
	defer r.wg.Done()
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("background: job %q panicked: %v\n%s", job.Name, rec, debug.Stack())
		}
	}()
	job.Run(ctx)
}
