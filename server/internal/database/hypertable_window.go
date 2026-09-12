package database

// LatestSampleWindow is the time window (30 minutes) applied to "most recent sample"
// lookups on metric hypertables (via `timestamp > NOW() - INTERVAL '30 minutes'`).
//
// A hypertable has no global index: each chunk carries its own. Without a
// predicate on the partitioning column the planner cannot exclude a single
// chunk, so a query that logically needs one row per host reads the entire
// retention window instead. The compression policies (migration 064, chunks
// older than 7 days) make that far worse: a compressed chunk no longer carries
// the original btree indexes, so every one of them has to go through a
// DecompressChunk node to be scanned at all.
//
// Both metric writers stay well inside this window — the agent's default
// report_interval is 30s and the Proxmox poller ticks every 30s — so a host or
// guest that is still reporting always has a sample in range. Callers that must
// keep serving a value for an entity which stopped reporting retry without the
// predicate when the bounded lookup comes back empty; that unbounded path is
// then only ever taken for entities that genuinely have no recent data, never
// for a live one.
//
// Note: queries embed `INTERVAL '30 minutes'` directly as static SQL string literals
// rather than dynamic concatenation to satisfy static analysis and linters.
const LatestSampleWindow = `INTERVAL '30 minutes'`

// ProxmoxSensorFreshness bounds the mapped-node sensor lookups
// (GetEffectiveHostCPUTemperature / GetEffectiveHostFanRPM) to 10 minutes.
// Those already discard any sample older than 10 minutes in Go; expressing
// the same cutoff in SQL keeps the result identical while letting the planner
// exclude chunks.
const ProxmoxSensorFreshness = `INTERVAL '10 minutes'`

