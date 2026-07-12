# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Local dev stack
```bash
docker compose up postgres          # PostgreSQL only
cd server && go run ./cmd/server    # API server on :8080
cd frontend && npm run dev          # Vite dev server on :3000 (proxies /api → :8080)
```

### Build
```bash
cd server   && go build ./...
cd agent    && bash build.sh v1.0.0   # multi-arch: amd64/arm64/armv7/armv6
cd frontend && npm run build
```

### Test & lint
```bash
cd server   && go test -v -race ./... && golangci-lint run   # v2.12.2, blocking in CI (server/.golangci.yml)
cd agent    && go test -v -race -coverprofile=coverage.out ./... && golangci-lint run   # agent/.golangci.yml
cd frontend && npm run typecheck && npm run lint
```

CI also blocks on `go mod tidy` cleanliness (both Go modules) and on `frontend/src/types/generated.ts` staying in sync with the Go models (`ci-server.yml` regenerates and diffs it) — see the generated-types note below before changing a model.

## Architecture

Three independent Go/Vue modules: **server** (Go API), **agent** (Go collector), **frontend** (Vue 3 SPA).

### Server (`server/`)

- Entry point: [cmd/server/main.go](server/cmd/server/main.go) — loads config, validates strict secrets, runs DB migrations, starts background goroutines + pollers, serves HTTP, handles graceful shutdown.
- [internal/api/router.go](server/internal/api/router.go) — only routes/middleware wiring. `SetupRouter(db, cfg, notifHub, bus, sched, dispatcher)` returns `(*gin.Engine, *ReleaseTrackerHandler, *ProxmoxHandler, *NPMHandler, cleanup func())` — the handlers whose pollers main starts, plus a cleanup fn that must be called on shutdown to stop the rate-limiter goroutines. `bus` is the [internal/events](server/internal/events/) pub/sub bus, threaded into the WS handler + the writer services so snapshots are pushed on write.
- [internal/api/middleware.go](server/internal/api/middleware.go) — JWT / CSRF / WS-token / rate limiter / API-key middlewares (CORS, security headers, request logger). WebSocket auth is cookie-first with an in-band `{"type":"auth","token":...}` JSON message fallback — the `?token=` query-string fallback was deliberately removed (it leaked into proxy/browser history logs); don't reintroduce it. `IPRateLimiter` is an in-process `map[string]*rate.Limiter` — **per-instance only, no shared state**. This is intentional and consistent with the rest of the server (the `internal/events` bus, the WS dashboard cache and every WS hub are equally in-process/single-instance), not an oversight to patch in isolation. If horizontal scaling of the server is ever needed, the rate limiter, the event bus and the WS hubs would all need to move to a shared backend (e.g. Redis pub/sub) together — swapping only the rate limiter would leave the rest broken under multi-instance while adding a new hard dependency for no coherent gain. Today's deployment model (`docker-compose.yml`) runs exactly one `server` container, so this does not apply yet.
- [internal/handlers/](server/internal/handlers/) — one or more files per domain (auth, hosts, agent, docker, apt, proxmox*, alert_rules, web_logs, ssl, uptime, …). Handlers translate HTTP; `respondError(c, err)` ([httperr.go](server/internal/handlers/httperr.go)) renders typed [apperr](server/internal/apperr/) errors uniformly. Background scheduling is **not** owned by handlers — they expose a poll-once op (`ProxmoxHandler.PollOnce`, `ReleaseTrackerHandler.CheckAll`) + `SetBackgroundContext(ctx)`, and [poller.Every](server/internal/poller/poller.go) (started from main) owns the loop.
- [internal/services/](server/internal/services/) — application/service layer, **one package per domain** (agent, alertrule, apt, audit, authn, docker, gitwebhook, host, hostperm, network, notifications, npm, proxmox, push, releasetracker, scheduledtask, settings, ssl, uptime, user, weblogs). Each owns its business logic behind a consumer-defined `Repository` port (satisfied structurally by `*database.DB`), so it is unit-testable without a database. **Every HTTP domain is migrated** — handlers are thin translators (bind → call service → `respondError`); the only `*database.DB` a handler still holds is for `requireHostAccess` ([host_authz.go](server/internal/handlers/host_authz.go)), the per-host authz guard that needs `*gin.Context`. Cross-package collaborators that need the concrete `*DB` (alert-engine preview, network snapshot builder, the relocated proxmox/release-tracker background pollers) are injected as funcs or hold `*DB` directly for background sync.
- [internal/events/](server/internal/events/) — tiny in-process pub/sub bus (zero deps → no import cycles, nil-safe) that drives the event-driven WS snapshots. Writers `Publish(topic)` after a state change; WS connections subscribe. See the `internal/ws` bullet.
- [internal/apperr/](server/internal/apperr/) — typed domain errors (`NotFound`/`Validation`/`Conflict`/`Forbidden`/`Unauthorized`/`TooManyRequests`/`BadGateway`/`Failed`/`Internal`) with a machine `Code` + HTTP status; `From()` coerces any error. `Internal` hides the message (generic 500); `Failed` keeps a human 500 message. Rendered as `{"error","code"}` by `respondError` at **every** handler error site.
- [internal/safego/](server/internal/safego/) — shared `Go` / `Recover` / `RecoverErr` helper: recovers a panic, logs it with a stack trace, and lets the process keep running. **Every detached goroutine** (pollers, WS fan-out, background jobs, notification dispatch) must go through it instead of a raw `go func(){...}()` — Go doesn't let a sibling goroutine recover another's panic, so an unprotected one takes down the whole server.
- [internal/poller/](server/internal/poller/) — generic `Every(ctx, interval, immediate, name, tick)` scheduling loop (wraps each tick in `safego`); stops on ctx cancellation.
- [internal/database/db.go](server/internal/database/db.go) — DB connection + embedded migration runner (`migrations/*.sql`). Migrations run in filename order on every startup; `schema_migrations` tracks applied ones. `000_full_baseline_breaking.sql` is the consolidated baseline — it declares its subsumed migrations via `-- ===== BEGIN <file>.sql =====` markers parsed by [migrations_baseline.go](server/internal/database/migrations_baseline.go).
- [internal/database/db_*.go](server/internal/database/) — DB methods split by domain (≈ 25 files). **All DB methods take `context.Context` as first arg.** Use `c.Request.Context()` from handlers; `context.Background()` only for background goroutines.
- [internal/models/](server/internal/models/) — shared structs **split per domain file** (alert.go, auth.go, command.go, docker.go, host.go, network.go, npm.go, proxmox.go, report.go, settings.go, synthetic.go, task.go, tracker.go, user.go, web_logs.go, webhook.go, ws.go). There is **no single `models.go`**.
- [internal/config/config.go](server/internal/config/config.go) — config loaded from env vars then overridden by `settings` table via `OverrideFromDB()`. The `DBSettingsLoader` interface avoids the database→config import cycle.
- [internal/alerts/](server/internal/alerts/) — evaluates alert rules and dispatches notifications, split into `engine.go` (orchestration) / `metrics.go` (metric resolvers) / `authfailures.go` (Proxmox syslog auth-failure parsing) / `severity.go` (hysteresis) / `notify.go` (dispatch). The exported entry points (`GetMetricValue`, `MatchRule`, `BuildDockerTestTargets`, `FetchProxmoxAuthFailureLogs`, `ResolveStaleIncidentsForRule`, `CurrentIncidentValue`) are injected into the alertrule/notifications services as funcs.
- [internal/ws/](server/internal/ws/) — `WSHandler` (per-page WebSocket endpoints), `CommandStreamHub` (live command output), `NotificationHub` (push-on-fire notifications). WS routes: `dashboard / hosts/:id / docker / network / apt / commands/stream/:id / notifications`. The snapshot endpoints (dashboard/host/docker/network/apt) are **event-driven**: each connection subscribes to its topic(s) on the [internal/events](server/internal/events/) bus and rebuilds+pushes (debounced ~750ms, diff-hash suppressed) when a writer publishes — agent ingest, the proxmox poller, the host-status monitor and apt command results all publish. A slow `snapshotSafetyInterval` (60s) rebuild is only a self-healing backstop, not the primary refresh path.
- [internal/background/](server/internal/background/) — `bg.Add(...)` jobs: audit cleanup, host status monitor, alert eval, metrics retention (tracker tag digests only — raw metric retention is owned by TimescaleDB policies), web-logs retention, uptime worker, SSL worker.
- [internal/dispatch/](server/internal/dispatch/) — server-side helper that persists `remote_commands` rows used by handlers + alert engine to queue agent commands.
- [internal/proxmoxclient/](server/internal/proxmoxclient/) — PVE HTTP API client (PVEAPIToken auth, optional TLS skip-verify).
- [internal/releasetracker/](server/internal/releasetracker/) — pure version-comparison helpers (`NormalizeDigest`, `IsVersionUpToDate`, `ResolveContainerVersion`). **Does not contain a Tracker** — the active release-tracking (CRUD + poll→detect→dispatch→notify pipeline) lives in [internal/services/releasetracker](server/internal/services/releasetracker/), behind a thin `ReleaseTrackerHandler`.
- [internal/synthetic/](server/internal/synthetic/) — uptime probe runner + SSL certificate checker (run from background jobs).
- [internal/gitprovider/](server/internal/gitprovider/) — GitHub / GitLab / Gitea release-API client (used by `ReleaseTrackerHandler`).
- [internal/scheduler/](server/internal/scheduler/) — cron-based `TaskScheduler` for scheduled-task executions.
- [internal/notify/](server/internal/notify/) — SMTP + ntfy senders + HTML alert email template.

### Agent (`agent/`)

- Entry point: [cmd/agent/main.go](agent/cmd/agent/main.go) — flag parsing, `--init` config generation, `--internal-update` self-update helper, main loop with sequential command worker.
- [internal/reporter/reporter.go](agent/internal/reporter/reporter.go) — collects metrics in parallel goroutines, builds `Report`, POSTs `/api/agent/report`, returns commands to the queue.
- [internal/dispatcher/](agent/internal/dispatcher/) — concurrent command runner with:
  - `dispatcher.go` — APT mutex + 4-slot semaphore for other modules + 45-min absolute timeout.
  - `registry.go` — module → handler map (`docker`, `compose`, `apt`, `journal`, `agent`, `systemd`, `processes`, `custom`, `crowdsec`).
  - `handler_<module>.go` — one file per module.
- [internal/sender/sender.go](agent/internal/sender/sender.go) — `Report` / `PendingCommand` / `CommandResult` structs + HTTP client (`X-API-Key` header, two timeouts: 30s reports / 30min commands).
- [internal/collector/](agent/internal/collector/) — one file per domain (system, docker, apt, disk, web_logs, systemd, journal, processes, crowdsec).
- [internal/config/tasks.go](agent/internal/config/tasks.go) — loads `tasks.yaml` (custom task definitions: id/command/timeout/env).

### Frontend (`frontend/src/`)

- [api/client.ts](frontend/src/api/client.ts) — shared Axios instance (`baseURL: /api`, `withCredentials: true`) + interceptors (CSRF double-submit via X-CSRF-Token, 401 → hard redirect to login). Endpoints live in per-domain modules (`auth.ts`, `hosts.ts`, `docker.ts`, `proxmox.ts`, `npm.ts`, `uptime.ts`, `ssl.ts`, …); [api/index.ts](frontend/src/api/index.ts) is a thin barrel that spreads them into one default export. **Add new endpoints to the relevant domain module, never to `index.ts`.**
- [router/index.ts](frontend/src/router/index.ts) — all routes use dynamic `import()` for lazy loading + chunk-retry logic on `ChunkLoadError`.
- [stores/](frontend/src/stores/) — Pinia (`auth`, `hosts`, `alertRules`, `dashboard`).
- [views/](frontend/src/views/) — one per route.
- [components/](frontend/src/components/) — organised by domain (alerts/, apt/, common/, dashboard/, disk/, docker/, host/, network/, proxmox/, security/, settings/, webhooks/) + a flat layer of generic components.
- [composables/](frontend/src/composables/) — `useWebSocket`, `useCommandStream`, `useDashboard`, `useHostDetail`, `useFormValidator`, `useToast`, `useConfirmDialog`, `usePagination`, …
- [utils/](frontend/src/utils/) — formatters, chart theme, dockerPorts, cron, dayjs, httpErrorBus, statusClasses, …

**TypeScript adoption**: all `.vue` files use `<script setup lang="ts">` (100% migrated). Composables, stores and `utils/` are `.ts`. The API + WebSocket payloads are fully typed against the generated models (see "Generated API types" below), including the dashboard / notification / IP-timeline consumers. One intentional residual remains: `useHostDetail.ts` keeps its 12 host-detail display refs as a local `AnyRecord` behind safe `asRecord`/`asRecordArray` coercers — the WS boundary feeding them is already typed (`WSHostSnapshot`), so this is a deliberate display-layer abstraction, not an unchecked wire contract. Typing it would cascade through the large `HostDetailView` + its sub-tabs for no contract-robustness gain.

### Agent ↔ Server protocol

Agents authenticate with a per-host API key sent in the `X-API-Key` header (cfg.APIKeyHeader). Keys are bcrypt-hashed at rest; agents are looked up via `db.GetHostByAPIKey`.

| Direction | Endpoint | Purpose |
|---|---|---|
| Agent → Server | `POST /api/agent/report` | Metrics, Docker state, APT status, web logs, disk health, capabilities |
| Server → Agent | Response body of `/report` | `{commands: []PendingCommand{ID, Module, Action, Target, Payload}, skip_metrics: bool}` |
| Agent → Server | `POST /api/agent/command/result` | `CommandResult{CommandID, Status, Output, AptStatus}` |
| Agent → Server | `POST /api/agent/command/stream` | Streaming chunk output for long commands |
| Agent → Server | `POST /api/agent/audit` | Autonomous actions (e.g. apt update on start) |

Commands persist in `remote_commands` (UUID PK, `module`, `action`, `target`, `payload` JSONB, `status`, `audit_log_id` FK to audit_logs ON DELETE SET NULL). Live command output streams over WebSocket at `GET /api/v1/ws/commands/stream/:id` (hub in [ws/command_stream.go](server/internal/ws/command_stream.go)).

The agent `Report` struct (`agent/internal/sender`) is fully typed against `agent/internal/collector` types; the server deserialises the same JSON into the strongly-typed `models.AgentReport`. The two modules share no code, so a **golden-fixture contract test** guards the wire format against silent drift: `protocol/agent_report.golden.json` is regenerated by the agent (`TestReportContractGolden`, reflection-filled) and decoded by the server with `DisallowUnknownFields` (`TestAgentReportContract`). When adding/renaming a report field, update both struct sets, regenerate the golden (`cd agent && go test ./internal/sender -run TestReportContractGolden -update`), and re-run the server test. See [protocol/README.md](protocol/README.md).

### Alert rules

Rules have a single `actions JSONB` column: `{channels: ["smtp","ntfy","browser","notify"], smtp_to, ntfy_topic, cooldown, command_trigger}`. The engine in `alerts/engine.go` iterates `rule.Actions.Channels` to dispatch. `source_type` is one of `agent | proxmox | synthetic`. Hysteresis: `threshold_warn / threshold_crit / threshold_clear_warn / threshold_clear_crit` per severity.

### Proxmox integration

`SetupRouter` returns the `ProxmoxHandler` so `main.go` can `StartPoller()` it (30s tick by default, respects per-connection `poll_interval_sec`). Token secrets are stored in `proxmox_connections.token_secret` and never returned to the frontend; retrieved only by poller/test via `GetProxmoxTokenSecret()`.

Guest ↔ host links live in `proxmox_guest_links` with `status: suggested|confirmed|ignored` and `metrics_source: auto|agent|proxmox`. When `metrics_source = proxmox` and Proxmox data is fresh, the server signals `skip_metrics: true` to the agent so it stops sending CPU/RAM. Sensor-source-providing hosts (cpu_temperature / fan_rpm) keep sending metrics regardless.

### Release trackers + Git webhooks

- `release_trackers` table: monitors GitHub/GitLab/Gitea releases **or** Docker registry digests. Detects new versions, then either notifies only (monitor-only) or dispatches a `module=custom` agent command. Handler: [handlers/release_trackers.go](server/internal/handlers/release_trackers.go). Background poller managed by `releaseTrackerH.StartPoller()`.
- `git_webhooks` table: public HMAC-authenticated endpoint `POST /api/v1/webhooks/git/:id/receive` (no JWT, dedicated 5 req/s rate limiter) that triggers a `module=custom` agent command. SS_REPO_NAME / SS_BRANCH / SS_COMMIT_SHA / SS_COMMIT_MESSAGE / SS_PUSHER are injected into the subprocess env.

### Key env vars

`JWT_SECRET`, `ADMIN_PASSWORD`, `DB_PASSWORD` are required for any non-trivial run. See [.env.example](.env.example) for the full list. `APP_ENV=dev` bypasses strict-secret validation for local development. The `settings` table can override most runtime config after first boot.

## Conventions & pitfalls

- **Generated API types are a source of truth, not a suggestion.** [frontend/src/types/generated.ts](frontend/src/types/generated.ts) is generated from the Go domain models (`server/internal/models`) by [tygo](https://github.com/gzuidhof/tygo) (config: [server/tygo.yaml](server/tygo.yaml)). Whenever a Go model changes, regenerate with `npm run gen:types` (or `cd server && go run github.com/gzuidhof/tygo@v0.2.21 generate`) and commit the result — `ci-server.yml` regenerates it and fails the build if your commit doesn't match, so this isn't optional. Per-domain files in `frontend/src/types/` re-export the generated types and add what generation can't express (status unions, response envelopes, request-vs-response shapes) — see [frontend/src/types/README.md](frontend/src/types/README.md).
- **Migrations are forward-only** — no `Down()`, no rollback tooling (`server/internal/database/migrations/*.sql`, baseline `000_full_baseline_breaking.sql`). Before a risky migration in production, snapshot first; a tested manual restore procedure (`pg_dump`/`pg_restore`) is in README's "Revenir en arrière après une mise à jour ratée" section.
- **Structured logging** (both modules) via `slog` — JSON in prod / text in dev on the server (`LOG_LEVEL`/`LOG_FORMAT`), text-for-journald by default on the agent. Use `slog.*Context(ctx, ...)` in new code, not the std `log` package or a bare `verbose bool` param.
- **Frontend tests**: Vitest unit/component tests (`npm run test`, `*.spec.ts`) run in happy-dom and run in CI. Real-browser tests (`npm run test:browser`, `*.browser.test.ts`, Chart.js/D3 rendering via Playwright/Chromium) do **not** run in CI — an upstream Vite 8 (rolldown) + Vitest 4 browser-mode bug breaks the dep optimizer (diagnosis in `vitest.browser.config.ts` and in `.github/workflows/ci-frontend.yml`). Run them locally (`npx playwright install --with-deps chromium && npm run test:browser`) before relying on them for a change that touches chart/map rendering.
- **PR titles are enforced, not just a convention.** [pr-checks.yml](.github/workflows/pr-checks.yml) blocks any PR whose title doesn't match Conventional Commits — `<type>(<scope>): <description>` with `type` one of `feat|fix|docs|style|refactor|perf|test|chore|ci|revert|build`, `(scope)` optional. A free-text or auto-generated title (e.g. "Add X") fails this check on open/edit — set a matching title before merging. The same workflow also auto-labels by changed path and warns (non-blocking) past 1000 changed lines.

See [server/CLAUDE.md](server/CLAUDE.md), [agent/CLAUDE.md](agent/CLAUDE.md) and [frontend/CLAUDE.md](frontend/CLAUDE.md) for conventions specific to each module. [AUDIT-2025.md](AUDIT-2025.md) is a point-in-time architecture audit (July 2026) kept for historical reference — its critical/important findings are resolved; don't treat it as current status.
