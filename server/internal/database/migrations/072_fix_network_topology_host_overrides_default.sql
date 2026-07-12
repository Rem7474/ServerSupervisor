-- host_overrides stores a JSON array of {hostId, ports} entries (same shape as
-- manual_services / excluded_ports), but the column was mistakenly given an
-- object default. On a fresh install with no saved topology config yet, the
-- frontend receives "{}" instead of "[]", assigns it to an array-typed ref,
-- and crashes NetworkView.vue with "hostPortConfig.value is not iterable" the
-- first time it iterates it (the config tab was never opened, so nothing had
-- saved a real array over the bad default yet).
ALTER TABLE network_topology_config ALTER COLUMN host_overrides SET DEFAULT '[]'::jsonb;
UPDATE network_topology_config SET host_overrides = '[]'::jsonb WHERE host_overrides = '{}'::jsonb;
