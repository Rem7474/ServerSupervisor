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

## Phase B — TODO

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
