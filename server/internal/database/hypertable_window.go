package database

// latestSampleWindow is the time predicate every "most recent sample" lookup on
// a hypertable must carry, appended as `WHERE ... timestamp > NOW() - ` + it.
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
const latestSampleWindow = `INTERVAL '30 minutes'`

// proxmoxSensorFreshness bounds the mapped-node sensor lookups
// (GetEffectiveHostCPUTemperature / GetEffectiveHostFanRPM). Those already
// discard any sample older than 10 minutes in Go; expressing the same cutoff in
// SQL keeps the result identical while letting the planner exclude chunks.
const proxmoxSensorFreshness = `INTERVAL '10 minutes'`
