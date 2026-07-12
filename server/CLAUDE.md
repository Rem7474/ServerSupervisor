# CLAUDE.md — server/

Scoped guidance for the Go API server. Read the root [CLAUDE.md](../CLAUDE.md) first — this file only covers conventions for **changing code in this directory**, not the architecture inventory (that's already listed there; don't duplicate it here).

## Adding or changing an HTTP endpoint

1. Add the method to the relevant `internal/services/<domain>.Repository` interface — only what the service actually needs, not a blanket passthrough of `*database.DB`. `*database.DB` satisfies every port structurally (no `implements` keyword), so a new method on `*database.DB` in `internal/database/db_<domain>.go` is enough to make it available.
2. Put the business logic in the service (`internal/services/<domain>/service.go`). Return `apperr.*` typed errors at decision points (`apperr.NotFound`, `.Validation`, `.Conflict`, `.Forbidden`, `.Unauthorized`, `.TooManyRequests`, `.BadGateway`, `.Failed`, `.Internal`) — don't let a bare `errors.New(...)` reach the handler if you can name what actually went wrong.
3. Keep the handler thin: bind the request, call the service with `c.Request.Context()`, `respondError(c, err)` on failure. Role checks (`c.GetString("role") != models.RoleAdmin`) belong in the handler, not the service — the service has no `*gin.Context`. The only handler that still holds a `*database.DB` directly is `requireHostAccess` ([host_authz.go](internal/handlers/host_authz.go)), because that per-host authz guard needs the `*gin.Context`; don't add a second one.
4. Register the route in `internal/api/router.go` — routing/grouping only, no logic. Look at an existing `register<Domain>Routes` function for the authenticated-vs-admin group split pattern before adding a new one.
5. If the response shape is a new or changed `internal/models/<domain>.go` struct, regenerate `frontend/src/types/generated.ts` (`npm run gen:types` from `frontend/`) and commit it — `ci-server.yml` fails the build otherwise (see root CLAUDE.md).

For a brand-new domain (not just a new endpoint on an existing one), follow the same shape as an existing small service (e.g. `internal/services/ssl` or `internal/services/uptime`): a `Repository` port, a `Service` struct built with `NewService(db, ...)`, a thin `Handler`, wired into `SetupRouter` alongside the others.

## Errors

Everything that reaches `respondError` should already be an `apperr.*` type. Untyped errors are coerced by `apperr.From()` into a generic `Internal` (message hidden from the client) — that's a safety net, not a substitute for typing the error where you know what it means. `%w`-wrapping deeper in the call stack is nice-to-have, not required: the HTTP boundary is the one place that matters for the client-visible envelope.

## Concurrency

Never start a bare `go func(){...}()`. Use `internal/safego` (`safego.Go`, `.Recover`, `.RecoverErr`) instead — Go has no way for one goroutine to recover another's panic, so an unprotected goroutine takes down the entire process, not just itself. Use `RecoverErr` specifically when the goroutine reports back over a channel another goroutine is blocked reading (e.g. a fan-out joined via `select` on N channels) — a bare `Recover` would just log and leave that reader blocked forever.

## Migrations

- `internal/database/migrations/*.sql`, embedded and applied in filename order at startup; applied ones are tracked in `schema_migrations`. **Forward-only** — there is no `Down()` and no rollback tooling.
- Name new migrations `NNN_description.sql` with the next free number (check the highest existing file first). `000_full_baseline_breaking.sql` is a squashed baseline of everything through migration 063 — it's not a template, don't imitate its shape.
- Never edit an already-applied migration file. Fix a mistake in a recent migration with a new follow-up migration instead (see `072_fix_network_topology_host_overrides_default.sql`, which corrects a wrong column default from an earlier migration).
- A migration that could destroy or reshape data should be tested against a real snapshot restore before it ships — see README's "Revenir en arrière après une mise à jour ratée" for the tested procedure.

## Tests

- Unit tests fake the `Repository` port — no database required.
- Integration tests use a real, ephemeral Postgres via `testcontainers-go` (`internal/testutil/postgres.go`); they skip cleanly with no Docker daemon and run for real in CI.
- `go test -v -race ./...` from `server/`. `golangci-lint run` (v2.12.2, config in `.golangci.yml`) is a second, separate, blocking CI gate — run it locally too, `go test` passing doesn't mean lint will.

## Don't

- Don't give a handler a `*database.DB` field for business logic — go through a service `Repository` port. `requireHostAccess` is the one sanctioned exception.
- Don't reintroduce the WebSocket `?token=` query-string auth fallback — removed deliberately (leaks into proxy/browser logs); cookie or the in-band `{"type":"auth"}` JSON message are the only supported paths.
- Don't hand-write a migration `Down()` or a rollback mechanism — the project's answer to a bad migration is a tested snapshot restore, not in-schema reversal.
