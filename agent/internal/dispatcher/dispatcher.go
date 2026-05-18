// Package dispatcher runs agent commands with concurrency controls.
// APT commands are serialized via a mutex (dpkg cannot run concurrently).
// All other modules share a 4-slot semaphore.
//
// Each module ("docker", "apt", "journal"…) is implemented in its own
// handler_<module>.go file and registered in registry.go's moduleRegistry.
// The dispatcher itself only owns the concurrency control + ctx lifecycle.
package dispatcher

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// maxCmdDuration is an absolute guard timeout applied to every command execution.
// Prevents a permanently-stuck subprocess (e.g. blocked apt upgrade) from leaking
// the goroutine indefinitely.
const maxCmdDuration = 45 * time.Minute

// UpdaterFunc starts a detached self-update helper process. Injected from the
// main package so the dispatcher does not need the HTTP/binary-install logic.
type UpdaterFunc func(s *sender.Sender, cmd sender.PendingCommand, cfgPath string) error

// Dispatcher executes agent commands with concurrency controls.
type Dispatcher struct {
	aptMu   sync.Mutex
	cmdSem  chan struct{}
	tasks   *config.TasksConfig
	cfg     *config.Config
	cfgPath string
	updater UpdaterFunc
}

// New returns a ready Dispatcher. updater is called for module=agent action=update.
func New(cfg *config.Config, cfgPath string, tasks *config.TasksConfig, updater UpdaterFunc) *Dispatcher {
	return &Dispatcher{
		cmdSem:  make(chan struct{}, 4),
		tasks:   tasks,
		cfg:     cfg,
		cfgPath: cfgPath,
		updater: updater,
	}
}

// Process runs each command in its own goroutine and waits for all to complete.
// APT commands serialise on aptMu (dpkg locks are exclusive); other modules
// share the 4-slot cmdSem.
func (d *Dispatcher) Process(s *sender.Sender, commands []sender.PendingCommand) {
	var wg sync.WaitGroup
	for _, cmd := range commands {
		wg.Add(1)
		go func(c sender.PendingCommand) {
			defer wg.Done()
			if c.Module == "apt" {
				d.aptMu.Lock()
				defer d.aptMu.Unlock()
			} else {
				d.cmdSem <- struct{}{}
				defer func() { <-d.cmdSem }()
			}
			d.execute(s, c)
		}(cmd)
	}
	wg.Wait()
}

// execute is the per-command entry point: it builds a bounded ctx and hands
// the command off to the registered module handler.
func (d *Dispatcher) execute(s *sender.Sender, cmd sender.PendingCommand) {
	// Background parent so commands survive agent shutdown; maxCmdDuration guards
	// against stuck subprocesses that would otherwise hold the goroutine forever.
	ctx, cancel := context.WithTimeout(context.Background(), maxCmdDuration)
	defer cancel()

	log.Printf("Processing command %s: module=%s action=%s target=%s", cmd.ID, cmd.Module, cmd.Action, cmd.Target)
	dispatch(ctx, d, s, cmd)
}
