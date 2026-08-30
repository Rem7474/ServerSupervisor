# CLAUDE.md — agent/

Scoped guidance for the Go collector/agent. Read the root [CLAUDE.md](../CLAUDE.md) first. This module is intentionally **not** structured like `server/` — it's a single-binary collector, not a services/repository-layered app; don't import that pattern here.

## Security model — read before touching `tasks.yaml` or the dispatcher

`tasks.yaml` is a local, per-host allowlist: id → argv + timeout, loaded from disk on the agent's own host. The server can trigger a task **by ID only** — it can never define what a task runs. This is deliberate, not a missing feature: it is the one guarantee that the server (shared, network-facing) cannot make an agent (with near-root access to a supervised host) execute arbitrary code. **Do not build a server→agent config-push endpoint for `tasks.yaml`** — that would hand the server exactly the power this mechanism exists to withhold from it. This was proposed and explicitly rejected in the last architecture audit (`AUDIT-2025.md`, roadmap item #10).

Custom task commands run as argv arrays via `exec.CommandContext`, never through a shell — don't change this to string-based shell execution.

## Dispatcher & concurrency

- `internal/dispatcher/dispatcher.go`: one mutex (`aptMu`) serializes every `apt` command (dpkg doesn't support concurrent invocations); a 4-slot semaphore bounds everything else; every command has a 45-minute absolute timeout (`maxCmdDuration`), with one deliberate exception: `module=restic action=run_backup` gets a 24h safety ceiling instead (`resticBackupMaxDuration`, see `cmdTimeout`), because a large backup can legitimately run far longer than 45 minutes as long as it keeps progressing. The real cutoff for a stuck restic backup is an idle-timeout watchdog inside `collector.RunResticBackupWithProgress` (shared with apt's `runCommandWithStreaming`/`aptCommandIdleTimeout` pattern), not a fixed duration — don't copy the 24h exception to another module without the same idle-watchdog justification.
- `handler_apt.go`'s post-command CVE re-enrichment (`collector.CollectAPT(ctx, true)` + `SendAptStatus`) runs in its own detached goroutine, *after* `handleApt` has already reported the command completed — not inline before returning. It used to run inline, still holding `aptMu` (`Process()` holds it around the whole `execute()`/`handleApt` call): on a fresh host with a large pending-package backlog, the sequential per-package security/CVE lookups (`apt-cache policy` + `apt-get changelog` + Ubuntu CVE API, one round-trip per package) could take many minutes, during which a *second* apt command would sit "pending" even though the first had genuinely finished. `CollectAPT` also now takes a `ctx` that bounds the whole call (`aptStatusRefreshTimeout`, 5 min) — the pending-package count/list is always fully computed first (pure parsing, no exec calls), only the finer security/CVE detail is skipped once that budget runs out. Don't move this back inline under `aptMu`, and don't drop the ctx bound.
- Separately, `handleApt` also calls `collector.CollectAPTFast(ctx)` (single bounded `apt-get upgrade --simulate` + parse, no per-package lookups at all) *synchronously*, before `ReportCommandResult`, and bundles the result into `CommandResult.AptStatus`. The server applies that field immediately on receiving the terminal report (`server/internal/services/agent/service.go`'s `if result.AptStatus != nil` block) — this is what makes the UI's pending-package count update the instant the command completes, rather than only ever being set by the slower detached-goroutine path above (which used to be the *only* thing populating `apt_status`, since `CommandResult.AptStatus` had a field defined for exactly this purpose but nothing ever set it). Keep `CollectAPTFast` free of per-package `apt-cache`/CVE calls — that's precisely the cost the detached goroutine above exists to keep off this synchronous path.
- `internal/dispatcher/registry.go`: module → handler map. Current modules: `docker`, `compose`, `apt`, `agent`, `systemd`, `processes`, `custom`, `crowdsec`, `journal`, `restic` — one `handler_<module>.go` per module. A new module needs both a registry entry and a `handler_<module>.go` (`handler_custom.go` is a reasonably minimal one to copy from).

## Collection must stay bounded

Every external command in `internal/collector/` goes through `exec.CommandContext` with an explicit timeout — never a bare `exec.Command`. The report loop runs as a single goroutine on a ticker that **drops** missed ticks rather than queuing them, so one hung command (a dead NFS/CIFS mount, a stuck `smartctl`) can silently stop all reporting forever if it isn't bounded. `internal/reporter/reporter.go` additionally wraps the whole parallel-collection phase in a `select`/`time.After`; keep that outer bound even when every individual collector already has its own timeout — it's the backstop for the one that doesn't.

## Config self-diagnostics (`internal/collector/diagnostics.go`)

`CheckConfig(cfg)` re-validates every *enabled* collector's prerequisites against what's actually on disk/in PATH — e.g. `collect_restic: true` with a `restic_conf_path` that doesn't exist, or `collect_web_logs: true` with globs matching no file. Called unconditionally every report cycle (cheap: `os.Stat`/`exec.LookPath` only, no network) and sent as `Report.Diagnostics`, which the server caches on `hosts.diagnostics` and the frontend surfaces as a banner on the host detail page (`HostDiagnosticsBanner.vue`). Predates this: `collector.CheckSMARTAvailability()`, which did the same thing for SMART alone but only logged at agent startup (`cmd/agent/main.go`) — never sent anywhere, so a config regression after boot (e.g. an unplugged USB SMART controller) went unnoticed. `CheckConfig` folds that same idea in for every optional collector and re-runs it continuously instead of once.

Two rules that keep this trustworthy signal instead of noise:
- **Only flag evidence of real misconfiguration, never "this optional field was left empty."** Most `restic_*`/`web_logs_*` keys are legitimate opt-outs; flagging every unset one would drown out the genuine problems. The one deliberate exception is `collect_restic: true` with `restic_conf_path` unset — that's exactly the "flag enabled, nothing else configured" case this was built for.
- **`Report.Diagnostics` has no `omitempty`, and `CheckConfig` always returns a non-nil slice (`[]DiagnosticIssue{}`, not `nil`, when clean).** Unlike `ResticProfiles` (only meaningfully present when configured), a fixed issue must actively clear server-side — an omitted/nil field on a "all clear" report would leave the stale warning in `hosts.diagnostics` forever. If you add a new field that mirrors "current problem state" rather than "optional discovered data," follow this pattern, not the Restic one.

Adding a new collector prerequisite (new `restic_*`/`collect_*` key, new required binary/path) should come with a matching `CheckConfig` branch — that's the whole point: catch it in the UI before the feature silently no-ops.

## Self-update (`cmd/agent/update.go`)

Flow: detached process via `systemd-run` → sha256 checksum verify → atomic `os.Rename` with the previous binary kept as `.bak` → `--internal-healthcheck` (a real collection-cycle self-test, not just `--version`) run against the **new** binary before touching the live service → `systemctl restart` → poll `systemctl is-active` → automatic rollback to `.bak` on failure at any step after the file swap. If you change what "the update succeeded" means, update the healthcheck, not just the smoke test — a binary that starts and parses flags but can't actually collect metrics must fail *here*, before it's live, not after.

## Web logs cursor (`internal/collector/web_logs.go`)

Fail-open by design: rotation, truncation, a missing/corrupt cursor file all degrade to "reprocess and possibly double-count," never to "stop collecting." Cursor writes are temp-file + `fsync` + atomic rename, not a direct `os.WriteFile`; rotation detection combines size **and** file identity (`os.SameFile`). Preserve both properties if you touch this file — a partial write or a size-only rotation check reintroduces the exact failure mode this was hardened against.

## Smaller packages

Listed so they aren't rediscovered as undocumented: `internal/agentws` (the optional WebSocket client for `/api/agent/ws` — receives only `{"type":"poll_now"}`, never command content; toggle with `disable_ws_push`), `internal/security` (the single authoritative list of "this key name holds a secret" patterns, shared by env-var filtering and YAML redaction — add to that list, don't grow a second one), `internal/logging` (slog setup, text-for-journald by default).

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
