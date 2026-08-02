# CLAUDE.md — agent/

Scoped guidance for the Go collector/agent. Read the root [CLAUDE.md](../CLAUDE.md) first. This module is intentionally **not** structured like `server/` — it's a single-binary collector, not a services/repository-layered app; don't import that pattern here.

## Security model — read before touching `tasks.yaml` or the dispatcher

`tasks.yaml` is a local, per-host allowlist: id → argv + timeout, loaded from disk on the agent's own host. The server can trigger a task **by ID only** — it can never define what a task runs. This is deliberate, not a missing feature: it is the one guarantee that the server (shared, network-facing) cannot make an agent (with near-root access to a supervised host) execute arbitrary code. **Do not build a server→agent config-push endpoint for `tasks.yaml`** — that would hand the server exactly the power this mechanism exists to withhold from it. This was proposed and explicitly rejected in the last architecture audit (`AUDIT-2025.md`, roadmap item #10).

Custom task commands run as argv arrays via `exec.CommandContext`, never through a shell — don't change this to string-based shell execution.

## Dispatcher & concurrency

- `internal/dispatcher/dispatcher.go`: one mutex serializes every `apt` command (dpkg doesn't support concurrent invocations); a 4-slot semaphore bounds everything else; every command has a 45-minute absolute timeout (`maxCmdDuration`), with one deliberate exception: `module=restic action=run_backup` gets a 24h safety ceiling instead (`resticBackupMaxDuration`, see `cmdTimeout`), because a large backup can legitimately run far longer than 45 minutes as long as it keeps progressing. The real cutoff for a stuck restic backup is an idle-timeout watchdog inside `collector.RunResticBackupWithProgress` (shared with apt's `runCommandWithStreaming`/`aptCommandIdleTimeout` pattern), not a fixed duration — don't copy the 24h exception to another module without the same idle-watchdog justification.
- `internal/dispatcher/registry.go`: module → handler map. Current modules: `docker`, `compose`, `apt`, `agent`, `systemd`, `processes`, `custom`, `crowdsec`, `journal`, `restic` — one `handler_<module>.go` per module. A new module needs both a registry entry and a `handler_<module>.go` (`handler_custom.go` is a reasonably minimal one to copy from).

## Collection must stay bounded

Every external command in `internal/collector/` goes through `exec.CommandContext` with an explicit timeout — never a bare `exec.Command`. The report loop runs as a single goroutine on a ticker that **drops** missed ticks rather than queuing them, so one hung command (a dead NFS/CIFS mount, a stuck `smartctl`) can silently stop all reporting forever if it isn't bounded. `internal/reporter/reporter.go` additionally wraps the whole parallel-collection phase in a `select`/`time.After`; keep that outer bound even when every individual collector already has its own timeout — it's the backstop for the one that doesn't.

## Self-update (`cmd/agent/update.go`)

Flow: detached process via `systemd-run` → sha256 checksum verify → atomic `os.Rename` with the previous binary kept as `.bak` → `--internal-healthcheck` (a real collection-cycle self-test, not just `--version`) run against the **new** binary before touching the live service → `systemctl restart` → poll `systemctl is-active` → automatic rollback to `.bak` on failure at any step after the file swap. If you change what "the update succeeded" means, update the healthcheck, not just the smoke test — a binary that starts and parses flags but can't actually collect metrics must fail *here*, before it's live, not after.

## Web logs cursor (`internal/collector/web_logs.go`)

Fail-open by design: rotation, truncation, a missing/corrupt cursor file all degrade to "reprocess and possibly double-count," never to "stop collecting." Cursor writes are temp-file + `fsync` + atomic rename, not a direct `os.WriteFile`; rotation detection combines size **and** file identity (`os.SameFile`). Preserve both properties if you touch this file — a partial write or a size-only rotation check reintroduces the exact failure mode this was hardened against.

## Protocol contract with the server

The agent and server share no code — the wire format is guarded by a golden-fixture contract test, not by shared types. If you add or rename a field on `Report` (`internal/sender`) or anything it embeds from `internal/collector`, regenerate `protocol/agent_report.golden.json` (`go test ./internal/sender -run TestReportContractGolden -update`) and update the mirrored struct in the server's `models.AgentReport`. See [protocol/README.md](../protocol/README.md) and the root CLAUDE.md's "Agent ↔ Server protocol" section.

## Build & release

- `build.sh` mirrors `.github/workflows/release.yml`'s 4-target matrix exactly (amd64, arm64, armv7 `GOARM=7`, armv6 `GOARM=6`). If you change one, change the other — they drifted before and it was a real source of confusion.
- Release binaries are sha256-checksummed and signed keylessly with cosign/Sigstore (GitHub OIDC, no key to manage) — verification commands are in each release's body.

## Tests

`go test -v -race -coverprofile=coverage.out ./...` from `agent/`. `golangci-lint run` (same v2.12.2 pin as `server/`, see `.golangci.yml`) is a separate, blocking CI gate — passing tests doesn't mean lint will pass too.

## Don't

- Don't add a way for the server to write or push `tasks.yaml` — see Security model above.
- Don't call an external command without `exec.CommandContext` and a real timeout.
- Don't treat self-update as fire-and-forget — it has an explicit rollback contract (`.bak` + pre-cutover healthcheck); preserve both if you touch `update.go`.
