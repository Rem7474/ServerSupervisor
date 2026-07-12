package poller

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvery_ImmediateAndTicked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count atomic.Int32
	ticks := make(chan struct{}, 10)
	Every(ctx, 20*time.Millisecond, true, "test", func(context.Context) {
		count.Add(1)
		select {
		case ticks <- struct{}{}:
		default:
		}
	})

	select {
	case <-ticks:
	case <-time.After(time.Second):
		t.Fatal("immediate tick never fired")
	}

	select {
	case <-ticks:
	case <-time.After(time.Second):
		t.Fatal("scheduled tick never fired")
	}

	if count.Load() < 2 {
		t.Errorf("tick count = %d, want at least 2", count.Load())
	}
}

func TestEvery_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32
	Every(ctx, 10*time.Millisecond, false, "test", func(context.Context) {
		count.Add(1)
	})

	time.Sleep(50 * time.Millisecond)
	cancel()
	afterCancel := count.Load()

	time.Sleep(100 * time.Millisecond)
	if got := count.Load(); got > afterCancel+1 {
		t.Errorf("tick count kept growing after ctx cancel: was %d right after cancel, now %d", afterCancel, got)
	}
}

// TestEvery_PanicInTickIsRecoveredAndLoopContinues guards the fix that wraps
// each tick with safego.Recover: a panicking tick must not kill the ticker
// loop (and, unrecovered, the whole process) — the next scheduled tick has to
// still fire.
func TestEvery_PanicInTickIsRecoveredAndLoopContinues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	done := make(chan struct{})
	Every(ctx, 15*time.Millisecond, false, "test", func(context.Context) {
		switch calls.Add(1) {
		case 1:
			panic("boom: simulated tick failure")
		case 2:
			close(done)
		}
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("no tick fired after the panicking one — Every did not recover and continue")
	}
}

// TestEvery_ImmediatePanicIsRecovered guards the immediate=true path, which
// runs on its own goroutine separate from the ticker loop.
func TestEvery_ImmediatePanicIsRecovered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	Every(ctx, time.Hour, true, "test", func(context.Context) {
		close(done)
		panic("boom: simulated immediate-tick failure")
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("immediate tick never fired")
	}
	// If the panic above were unrecovered it would crash the test binary
	// before reaching this point.
}
