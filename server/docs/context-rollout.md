# C4 — `context.Context` rollout plan

## Phase A — DONE

Every public method on `*database.DB` now accepts a `context.Context` as its
first argument and uses `QueryContext` / `QueryRowContext` / `ExecContext` /
`BeginTx` internally. The `Query`, `QueryRow`, `Exec` helpers on `*DB`
follow the same convention.

Side effects of Phase A:

- The `config.DBSettingsLoader` interface now takes `ctx`.
- The scheduler's `DB` interface now takes `ctx`.
- All call sites outside the database package were mechanically updated to
  pass `context.Background()` as a temporary placeholder. There are exactly
  **`context.Background()`** sentinels seeded across the server packages —
  use `grep -rn "context.Background()"` to find them.

The whole server still builds (`go build ./...`) and tests still pass
(`go test ./...`).

## Phase B — DONE

A root `ctx` is now created in `cmd/server/main.go` via
`signal.NotifyContext(ctx, SIGINT, SIGTERM)` and threaded through:

- `background.Runner.Start(ctx)` — every background job inherits it
- `scheduler.TaskScheduler.Start(ctx)` — cron jobs' DB calls use it
- `ProxmoxHandler.StartPoller(ctx)` / `ReleaseTrackerHandler.StartPoller(ctx)`
- `alerts.EvaluateAlerts(ctx, ...)` and every helper in `alerts/engine.go`
- `dispatch.Dispatcher.Create(ctx, req)` — all call sites updated
- `networkview.BuildSnapshot(ctx, db)`
- `ws.snapshots` and `version_compare` use the request ctx via Gin handlers

HTTP handlers now uniformly use `c.Request.Context()`. Fire-and-forget
goroutines started from handlers (release tracker check-now, Proxmox poll-now,
agent command completion callbacks) use a long-lived `pollerCtx` / `bgCtx`
field on the handler struct, set when the poller starts. Cancelling the root
ctx (SIGINT/SIGTERM) propagates to every in-flight DB call.

Remaining `context.Background()` occurrences are intentional and limited to:

- Struct field placeholders (`pollerCtx: context.Background()`) — overwritten
  at startup.
- Test helpers (`testutil/postgres.go`, `_test.go` files) — Phase B exempts
  tests; `t.Context()` works fine as a follow-up.
- Database migration runner (`database/db.go`) — runs once at startup, before
  the root ctx is fully wired, and must complete unconditionally.
- `config.OverrideFromDB` — startup-only.

## Phase B — historical (kept for reference)

Replace every `context.Background()` placeholder by a context that propagates
cancellation correctly:

| Caller type                 | Context to thread                                                              |
|-----------------------------|--------------------------------------------------------------------------------|
| HTTP handler (gin)          | `c.Request.Context()` — cancels when the client disconnects                    |
| WebSocket upgrade handler   | `c.Request.Context()` before the upgrade; a `context.WithCancel` after         |
| Background goroutine        | A long-lived `ctx` created in `cmd/server/main.go` and cancelled on shutdown   |
| Scheduler / cron jobs       | A scheduler-owned `ctx` (from `*TaskScheduler.Start`)                          |
| Agent ingest / dispatcher   | The HTTP request context                                                       |
| Tests                       | `t.Context()` (Go 1.24+) or a fresh `context.Background()`                     |

### Suggested rollout

1. In `cmd/server/main.go`, build a root `ctx, cancel := signal.NotifyContext(...)`.
2. Pass that root ctx to background jobs, the scheduler, the GitHub tracker,
   and the Proxmox poller.
3. In each handler file, switch `context.Background()` to `c.Request.Context()`.
4. In each background package (`audit.go`, `hosts.go`, `metrics.go`, …) inject
   the job context.
5. Drop the `context.Background()` placeholders to zero with
   `grep -rn "context.Background()" server/internal/`.

### Acceptance criteria

- `grep -rn "context.Background()" server/internal/handlers/` returns 0 hits.
- A SIGTERM during a long Proxmox poll cancels the in-flight query rather than
  letting it run to completion.
- `go test -race ./...` still green.
