-- Migration 092: network_flow_metrics — per-host "top talkers" (remote IP,
-- port, protocol, best-effort process attribution), agent-aggregated from
-- conntrack accounting (see agent/internal/collector/network_flows.go). One
-- row per (host, report cycle, top-N talker), plus a single "others" rollup
-- row per cycle (is_others = true) for everything beyond the agent's top-N
-- cutoff — the agent bounds the payload, not the server.
--
-- rx_bytes/tx_bytes/packets are per-cycle DELTAS, not conntrack's cumulative
-- counters — history aggregation must SUM() these per time_bucket, never
-- MAX()-MIN() like system_metrics' truly-cumulative network_rx_bytes.
--
-- remote_ip uses the same VARCHAR(45) convention as hosts.ip_address (fits
-- IPv6), not the INET type — unused elsewhere in this schema, no reason to
-- introduce it here.
--
-- No add_retention_policy() here, deliberately: remote_ip is a potentially
-- identifying value (RGPD), so retention is a configurable applicative job
-- (NetworkFlowsRetentionDays, internal/background/network_flows.go) mirroring
-- web_log_requests/WebLogsRetentionDays, not a fixed Timescale policy like
-- system_metrics/disk_metrics.
--
-- New table (this feature has no pre-Timescale history), so it's created
-- directly and converted to a hypertable in the same migration — gated on
-- TimescaleDB availability like migration 064, to stay usable on a plain
-- PostgreSQL dev/test instance.

CREATE TABLE IF NOT EXISTS network_flow_metrics (
    id           BIGSERIAL,
    host_id      VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    "timestamp"  TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_others    BOOLEAN NOT NULL DEFAULT false,
    remote_ip    VARCHAR(45) DEFAULT '',
    remote_port  INTEGER NOT NULL DEFAULT 0,
    protocol     VARCHAR(8) DEFAULT '',
    direction    VARCHAR(10) DEFAULT '',
    process_name VARCHAR(255) DEFAULT '',
    pid          INTEGER NOT NULL DEFAULT 0,
    rx_bytes     BIGINT NOT NULL DEFAULT 0,
    tx_bytes     BIGINT NOT NULL DEFAULT 0,
    packets      BIGINT NOT NULL DEFAULT 0,
    connections  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (id, "timestamp")
);

CREATE INDEX IF NOT EXISTS idx_network_flow_metrics_host_ts
    ON network_flow_metrics (host_id, "timestamp" DESC);
CREATE INDEX IF NOT EXISTS idx_network_flow_metrics_host_remote
    ON network_flow_metrics (host_id, remote_ip, "timestamp" DESC)
    WHERE is_others = false;

DO $$
DECLARE
  tsdb_available BOOLEAN := FALSE;
BEGIN
  SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')
    INTO tsdb_available;

  IF NOT tsdb_available THEN
    RAISE NOTICE 'TimescaleDB not available; network_flow_metrics stays a plain table.';
    RETURN;
  END IF;

  CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

  IF NOT EXISTS (SELECT 1 FROM timescaledb_information.hypertables
                 WHERE hypertable_name = 'network_flow_metrics') THEN
    PERFORM create_hypertable('network_flow_metrics', 'timestamp', migrate_data => true);
    ALTER TABLE network_flow_metrics
      SET (timescaledb.compress, timescaledb.compress_segmentby = 'host_id');
    PERFORM add_compression_policy('network_flow_metrics', INTERVAL '3 days');
  END IF;
END $$;
