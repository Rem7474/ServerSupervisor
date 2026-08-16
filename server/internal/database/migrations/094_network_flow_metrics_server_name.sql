-- Migration 093: network_flow_metrics.server_name — the TLS SNI hostname
-- observed on an outbound flow, so a talker can read as "github.com" instead of
-- just "140.82.121.4:443".
--
-- Only ever populated when the agent's optional, off-by-default L7 capture is
-- enabled (network_flows_l7_capture, requires CAP_NET_RAW — see
-- agent/internal/collector/network_flows_l7.go). Every existing row and every
-- agent that doesn't opt in leaves it '', which the frontend renders by falling
-- back to its own well-known-port heuristic — so this column is purely additive
-- and needs no backfill.
--
-- VARCHAR(253) is the maximum length of a DNS name (RFC 1035); an SNI value
-- longer than that is malformed and gets rejected at insert rather than
-- silently stored.
--
-- ADD COLUMN with a constant default is metadata-only on PostgreSQL 11+, so
-- this is safe on a large hypertable without a rewrite.

ALTER TABLE network_flow_metrics
    ADD COLUMN IF NOT EXISTS server_name VARCHAR(253) NOT NULL DEFAULT '';
