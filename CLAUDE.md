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
cd server   && go test -v -race ./...
cd agent    && go test -v -race -coverprofile=coverage.out ./...
cd frontend && npm run typecheck && npm run lint
```

## Architecture

Three independent Go/Vue modules: **server** (Go API), **agent** (Go collector), **frontend** (Vue 3 SPA).

### Server (`server/`)

- Entry point: [cmd/server/main.go](server/cmd/server/main.go) — loads config, validates strict secrets, runs DB migrations, starts background goroutines + pollers, serves HTTP, handles graceful shutdown.
- [internal/api/router.go](server/internal/api/router.go) — only routes/middleware wiring. `SetupRouter` returns `(*gin.Engine, *ReleaseTrackerHandler, *ProxmoxHandler, cleanup func())`. The cleanup fn must be called on shutdown to stop the rate-limiter goroutines.
- [internal/api/middleware.go](server/internal/api/middleware.go) — JWT / CSRF / WS-token / rate limiter / API-key middlewares (CORS, security headers, request logger).
- [internal/handlers/](server/internal/handlers/) — one or more files per domain (auth, hosts, agent, docker, apt, proxmox*, alert_rules, web_logs, ssl, uptime, …). Long-lived handlers (`ProxmoxHandler` 30s poller, `ReleaseTrackerHandler` GitHub poller) own their own goroutines.
- [internal/database/db.go](server/internal/database/db.go) — DB connection + embedded migration runner (`migrations/*.sql`). Migrations run in filename order on every startup; `schema_migrations` tracks applied ones. `000_full_baseline_breaking.sql` is the consolidated baseline — it declares its subsumed migrations via `-- ===== BEGIN <file>.sql =====` markers parsed by [migrations_baseline.go](server/internal/database/migrations_baseline.go).
- [internal/database/db_*.go](server/internal/database/) — DB methods split by domain (≈ 25 files). **All DB methods take `context.Context` as first arg.** Use `c.Request.Context()` from handlers; `context.Background()` only for background goroutines.
- [internal/models/](server/internal/models/) — shared structs **split per domain file** (alert.go, auth.go, command.go, docker.go, host.go, network.go, proxmox.go, report.go, synthetic.go, task.go, tracker.go, user.go, web_logs.go, webhook.go). There is **no single `models.go`**.
- [internal/config/config.go](server/internal/config/config.go) — config loaded from env vars then overridden by `settings` table via `OverrideFromDB()`. The `DBSettingsLoader` interface avoids the database→config import cycle.
- [internal/alerts/engine.go](server/internal/alerts/engine.go) — evaluates alert rules and dispatches notifications. Note: **monolithic (1500+ lines)** — flagged for split into engine / metric resolvers / notify.
- [internal/ws/](server/internal/ws/) — `WSHandler` (per-page WebSocket endpoints), `CommandStreamHub` (live command output), `NotificationHub` (push-on-fire notifications). WS routes: `dashboard / hosts/:id / docker / network / apt / commands/stream/:id / notifications`.
- [internal/background/](server/internal/background/) — `bg.Add(...)` jobs: audit cleanup, host status monitor, alert eval, metrics retention (tracker tag digests only — raw metric retention is owned by TimescaleDB policies), web-logs retention, uptime worker, SSL worker.
- [internal/dispatch/](server/internal/dispatch/) — server-side helper that persists `remote_commands` rows used by handlers + alert engine to queue agent commands.
- [internal/proxmoxclient/](server/internal/proxmoxclient/) — PVE HTTP API client (PVEAPIToken auth, optional TLS skip-verify).
- [internal/releasetracker/](server/internal/releasetracker/) — pure version-comparison helpers (`NormalizeDigest`, `IsVersionUpToDate`, `ResolveContainerVersion`). **Does not contain a Tracker** — the active release-tracking lives in [handlers/release_trackers.go](server/internal/handlers/release_trackers.go).
- [internal/synthetic/](server/internal/synthetic/) — uptime probe runner + SSL certificate checker (run from background jobs).
- [internal/gitprovider/](server/internal/gitprovider/) — GitHub / GitLab / Gitea release-API client (used by `ReleaseTrackerHandler`).
- [internal/scheduler/](server/internal/scheduler/) — cron-based `TaskScheduler` for scheduled-task executions.
- [internal/notify/](server/internal/notify/) — SMTP + ntfy senders + HTML alert email template.

### Agent (`agent/`)

- Entry point: [cmd/agent/main.go](agent/cmd/agent/main.go) — flag parsing, `--init` config generation, `--internal-update` self-update helper, main loop with sequential command worker.
- [internal/reporter/reporter.go](agent/internal/reporter/reporter.go) — collects metrics in parallel goroutines, builds `Report`, POSTs `/api/agent/report`, returns commands to the queue.
- [internal/dispatcher/](agent/internal/dispatcher/) — concurrent command runner with:
  - `dispatcher.go` — APT mutex + 4-slot semaphore for other modules + 45-min absolute timeout.
  - `registry.go` — module → handler map (`docker`, `apt`, `journal`, `agent`, `systemd`, `processes`, `custom`, `crowdsec`).
  - `handler_<module>.go` — one file per module.
- [internal/sender/sender.go](agent/internal/sender/sender.go) — `Report` / `PendingCommand` / `CommandResult` structs + HTTP client (`X-API-Key` header, two timeouts: 30s reports / 30min commands).
- [internal/collector/](agent/internal/collector/) — one file per domain (system, docker, apt, disk, web_logs, systemd, journal, processes, crowdsec).
- [internal/config/tasks.go](agent/internal/config/tasks.go) — loads `tasks.yaml` (custom task definitions: id/command/timeout/env).

### Frontend (`frontend/src/`)

- [api/index.ts](frontend/src/api/index.ts) — single Axios client (`baseURL: /api`, `withCredentials: true`) + monolithic default-export of all API methods. Handles CSRF double-submit (X-CSRF-Token) and 401 → hard redirect to login.
- [router/index.ts](frontend/src/router/index.ts) — all routes use dynamic `import()` for lazy loading + chunk-retry logic on `ChunkLoadError`.
- [stores/](frontend/src/stores/) — Pinia (`auth`, `hosts`, `alertRules`, `dashboard`).
- [views/](frontend/src/views/) — one per route.
- [components/](frontend/src/components/) — organised by domain (alerts/, apt/, common/, dashboard/, disk/, docker/, host/, network/, proxmox/, security/, settings/, webhooks/) + a flat layer of generic components.
- [composables/](frontend/src/composables/) — `useWebSocket`, `useCommandStream`, `useDashboard`, `useHostDetail`, `useFormValidator`, `useToast`, `useConfirmDialog`, `usePagination`, …
- [utils/](frontend/src/utils/) — formatters, chart theme, dockerPorts, cron, dayjs, httpErrorBus, statusClasses, …

**TypeScript adoption**: all `.vue` files use `<script setup lang="ts">` (100% migrated). Composables, stores and `utils/` are `.ts`. Some `as any` casts and `any` typings remain at composable↔component frontiers (most concentrated in `useDashboard.ts`, `useHostDetail.ts`, `useGitWebhooksPage.ts` and the views that consume them) — runtime is correct but those points are not type-checked.

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

## Known refactor debt

- **Phase B context.Context** — DONE. A root ctx (SIGINT/SIGTERM-bound) is threaded through background jobs, scheduler, pollers, the alert engine, dispatch, networkview and ws; HTTP handlers use `c.Request.Context()` ([docs](server/docs/context-rollout.md)). Remaining `context.Background()` are intentional (struct placeholders, startup migrations, tests).
- **Structured logging** — `internal/logging` wires slog (JSON in prod / text in dev) as the default and bridges the std `log` package through it. `RequestIDMiddleware` assigns a correlation ID per request (X-Request-ID header) that the contextHandler attaches to every log line. Configure via `LOG_LEVEL` / `LOG_FORMAT`. Direct `slog.{Info,Warn,Error}Context(ctx, …)` migration is DONE across `scheduler/`, `alerts/`, `handlers/`, `database/`, `dispatch/`, `notify/`, `ws/`. The `log.SetOutput` bridge in `logging.go` remains for any third-party or future call sites that use the std package.
- **Monolithic files** flagged for split:
  - server: DONE. `alerts/engine.go` split into `engine.go` (orchestration) / `metrics.go` (metric resolvers) / `authfailures.go` (Proxmox syslog auth-failure parsing) / `severity.go` (hysteresis) / `notify.go` (dispatch). `handlers/alert_rules.go` split into `alert_rules.go` (struct + vars + validation) / `_capabilities.go` / `_crud.go` / `_testrun.go` / `_incidents.go`. `handlers/release_trackers.go` split into `release_trackers.go` (lifecycle) / `_poller.go` / `_dispatch.go` / `_notify.go` / `_http.go`. Each split file is same-package; no behavior change. Remaining: [handlers/agent.go](server/internal/handlers/agent.go) (`ReceiveReport` ingest ~250 lines).
  - frontend: DONE. `AlertRuleModal.vue` (1321→712) split into `AlertRuleStepSource/StepConditions/StepNotifications.vue` (TS). `TrafficView.vue` (1522→1191) split into `components/security/Traffic{KpiCards,WorldMap,RequestsChart,StatusChart}.vue` (TS). `ProxmoxNodeView.vue` (2629→1319) split into `components/proxmox/{GuestLinkCell,ProxmoxNodeDisksTab,ProxmoxNodeStorageTab,ProxmoxNodeTasksTab,ProxmoxNodeUpdatesTab,ProxmoxNodeServicesTab,ProxmoxNodeSecurityTab,ProxmoxNodeGuestsTab}.vue` (TS) + the pre-existing `ProxmoxNodeChartsPanel`. Sub-components own their data via props and raise actions via emits; the parent keeps orchestration (fetch/state/polling).
- **API client** — [api/index.ts](frontend/src/api/index.ts) is now a thin barrel: shared axios instance + interceptors live in [api/client.ts](frontend/src/api/client.ts), endpoints are grouped in per-domain modules (`auth.ts`, `hosts.ts`, `docker.ts`, `proxmox.ts`, …), and `index.ts` re-assembles the default export by spreading them. Add new endpoints to the relevant domain module.
- **TypeScript migration** — DONE. All `.vue` files use `<script setup lang="ts">`. New code should also be TS. Residual `any` exists at composable↔component frontiers (see TypeScript adoption note above); tightening those is a follow-up sprint, not a blocker.
- **Generated API types** — [frontend/src/types/generated.ts](frontend/src/types/generated.ts) is generated from the Go domain models (`server/internal/models`) by [tygo](https://github.com/gzuidhof/tygo) (config: [server/tygo.yaml](server/tygo.yaml)). Regenerate with `npm run gen:types` (or `cd server && go run github.com/gzuidhof/tygo@v0.2.21 generate`) and commit it whenever a model changes — it is the single source of truth for API shapes. Per-domain files in `types/` re-export the generated types and add what generation can't express (status unions, response envelopes, flattened embeds, request-vs-response shapes). API methods in `api/*.ts` are typed via `api.get<T>()`. See [frontend/src/types/README.md](frontend/src/types/README.md).
- **Frontend tests** — Vitest. Unit/component tests run in happy-dom (`npm run test`, `*.spec.ts` co-located). Real-browser tests (Chart.js/D3 rendering) run in Chromium via Playwright (`npm run test:browser`, `*.browser.test.ts`, config `vitest.browser.config.ts`). CI must `npx playwright install --with-deps chromium` before `test:browser`. Test artifacts (`__screenshots__/`, `.vitest-attachments/`) are gitignored.
