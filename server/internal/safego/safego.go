// Package safego centralizes goroutine panic recovery. A panic in an
// unprotected goroutine crashes the whole process (Go does not let a sibling
// goroutine catch it), which is rarely the intent for a fire-and-forget
// background fan-out or poller tick. Use Go to spawn a self-contained
// goroutine, or Recover/RecoverErr to guard the body of a goroutine whose
// lifecycle (sync.WaitGroup, channel) is already managed by the caller.
package safego

import (
	"context"
	"log/slog"
	"runtime/debug"
)

// Go runs fn in a new goroutine, recovering and logging any panic instead of
// letting it crash the process.
func Go(ctx context.Context, name string, fn func()) {
	go func() {
		defer Recover(ctx, name)
		fn()
	}()
}

// Recover is a deferred panic handler: it logs the panic (with a name label
// and stack trace) and swallows it so the goroutine returns normally instead
// of crashing the process. Use it via `defer safego.Recover(ctx, name)` at
// the top of a goroutine body whose completion is tracked another way
// (sync.WaitGroup.Done, closing a channel, ...).
func Recover(ctx context.Context, name string) {
	if rec := recover(); rec != nil {
		logPanic(ctx, name, rec)
	}
}

// RecoverErr behaves like Recover but also returns the recovered value (nil
// when no panic occurred), so a caller that joins goroutines over result
// channels can turn the panic into an error instead of blocking forever
// waiting for a message that a silently-recovered goroutine will never send.
func RecoverErr(ctx context.Context, name string) any {
	if rec := recover(); rec != nil {
		logPanic(ctx, name, rec)
		return rec
	}
	return nil
}

func logPanic(ctx context.Context, name string, rec any) {
	slog.ErrorContext(ctx, "goroutine panicked",
		slog.String("name", name),
		slog.Any("panic", rec),
		slog.String("stack", string(debug.Stack())))
}
