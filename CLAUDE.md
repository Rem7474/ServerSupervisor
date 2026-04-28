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

Three independent modules: **server** (Go), **agent** (Go), **frontend** (Vue 3 SPA).

### Server (`server/`)
- Entry point: `cmd/server/main.go` — loads config, runs DB migrations, starts all background goroutines, then serves HTTP.
- `internal/api/router.go` — all Gin routes and middleware registration.
- `internal/database/db.go` — DB connection + embedded migration runner (`migrations/*.sql`). Migrations run in filename order on every startup via `schema_version` tracking. All DB methods live in `db.go` and `db_*.go` domain files.
- `internal/models/models.go` — all shared structs between packages.
- `internal/config/config.go` — config loaded from env vars, then overridden by DB settings table at startup (`OverrideFromDB()`). The `DBSettingsLoader` interface avoids import cycles.
- `internal/handlers/` — long-lived stateful handlers (ProxmoxHandler with 30s poller, ReleaseTrackerHandler with GitHub poller).
- `internal/alerts/engine.go` — evaluates alert rules and dispatches notifications (smtp/ntfy/browser channels).
- Background goroutines started in `main.go`: audit cleanup, host status monitor, alert engine, metrics downsampling, metrics/web-logs retention.

### Agent (`agent/`)
- Entry point: `cmd/agent/main.go` — collects data every 30s, POSTs to `/api/agent/report`, processes commands from the response.
- `internal/collector/` — one file per domain (system, docker, apt, disk, web_logs, etc.).
- `internal/config/tasks.go` — loads `tasks.yaml` (custom task definitions with id/command/timeout).
- Command dispatch: `processCommands()` in `main.go` switches on `PendingCommand.Module` → `docker|apt|systemd|journal|processes|custom|agent`.

### Frontend (`frontend/src/`)
- `api/index.js` — single Axios client for all HTTP calls.
- `router/index.ts` — all routes use dynamic `import()` for lazy loading. Includes chunk-retry logic on `ChunkLoadError`.
- `stores/` — Pinia stores (auth token/role, hosts).
- Views in `views/`, reusable pieces in `components/`.

### Agent ↔ Server protocol
Agents authenticate with a per-host API key (`Authorization: Bearer <key>`, bcrypt-hashed in DB).

| Direction | Endpoint | Purpose |
|---|---|---|
| Agent → Server | `POST /api/agent/report` | Metrics, Docker state, APT status, web logs |
| Server → Agent | Response body of `/report` | `commands: []PendingCommand{ID, Module, Action, Target, Payload}` |
| Agent → Server | `POST /api/agent/command/result` | `CommandResult{CommandID, Status, Output}` |
| Agent → Server | `POST /api/agent/command/stream` | Streaming output for long commands |
| Agent → Server | `POST /api/agent/audit` | Autonomous actions (e.g. apt update on start) |

Commands are persisted in the `remote_commands` table (UUID PK, `module`, `action`, `target`, `payload` JSONB, `status`, `audit_log_id`). The WebSocket stream for a running command is at `GET /api/v1/ws/commands/stream/:id` (hub in `internal/api/command_stream.go`).

### Alert rules
Rules have a single `actions JSONB` column: `{channels: ["smtp","ntfy","browser","notify"], smtp_to, ntfy_topic, cooldown}`. The engine in `alerts/engine.go` iterates `rule.Actions.Channels` to dispatch. Frontend always sends the full `actions` object on create/update.

### Proxmox integration
`SetupRouter` returns `(*gin.Engine, *ReleaseTrackerHandler, *ProxmoxHandler)` — the Proxmox handler owns the 30s poll loop. Token secrets are stored in DB and never returned to the frontend; retrieved only by poller/test via `GetProxmoxTokenSecret()`.

### Key env vars
`JWT_SECRET`, `ADMIN_PASSWORD`, `DB_PASSWORD` are required for any non-trivial run. See `.env.example` for the full list. DB settings table can override most runtime config after first boot.
