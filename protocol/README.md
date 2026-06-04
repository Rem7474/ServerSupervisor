# Agent ↔ Server protocol contract

The agent (`agent/`) and server (`server/`) are independent Go modules with **no
shared code**. The agent serialises its periodic report from `sender.Report`
(types in `agent/internal/sender` + `agent/internal/collector`); the server
deserialises the same JSON into `models.AgentReport`
(`server/internal/models`). Historically the two struct sets drifted silently —
a renamed or missing JSON tag on the server made the agent's data vanish on
ingest with no error (e.g. `realloc_sectors`, `power_cycles`, CrowdSec decision
`type`).

`agent_report.golden.json` is the single source-of-truth fixture that pins the
wire shape and makes any drift a test failure.

## How it works

- **Agent side** — `agent/internal/sender/contract_test.go`
  (`TestReportContractGolden`) fills a `Report` by reflection so **every**
  exported field (including `omitempty` ones) is set, marshals it, and compares
  it to `agent_report.golden.json`. If you add/rename/remove a field on any
  collector type, this test fails until the golden is regenerated — so the
  golden always reflects exactly what the agent emits.

- **Server side** — `server/internal/handlers/agent_contract_test.go`
  (`TestAgentReportContract`) decodes the same golden into `models.AgentReport`
  with `json.Decoder.DisallowUnknownFields()`. If the agent emits a key the
  server does not model (missing field **or** mismatched json tag, at any
  nesting depth), the decode fails. A handful of historically-fragile deep
  fields are also explicitly spot-checked for non-zero values.

Both tests run in normal CI (`go test ./...` per module) — no extra wiring.

## Workflow when changing the protocol

1. Change the agent collector/sender struct(s) and/or the server model(s).
2. Regenerate the golden from the agent module:
   ```bash
   cd agent && go test ./internal/sender -run TestReportContractGolden -update
   ```
3. Run the server contract test:
   ```bash
   cd server && go test ./internal/handlers -run TestAgentReportContract
   ```
   If it fails, the server model is missing the new/renamed field — add it, then
   re-run. Commit the regenerated `agent_report.golden.json` alongside the code
   change.

The golden is intentionally committed: a diff to it in a PR is the human-visible
signal that the agent↔server wire format changed.
