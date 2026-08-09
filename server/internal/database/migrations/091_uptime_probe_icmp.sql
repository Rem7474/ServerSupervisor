-- Generic ICMP (ping) checks (ROADMAP.md item #8) — a third uptime_probes
-- type alongside the existing http/tcp, for equipment that isn't
-- agent-installable and doesn't expose a TCP/HTTP port to probe (a switch,
-- a printer, a raw IP camera, ...). Reuses the existing probe/result
-- schema unchanged (success/latency_ms/error already generic enough for a
-- single echo request's outcome) — see internal/synthetic/uptime.go's
-- checkICMP. Real ICMP requires CAP_NET_RAW; see server/Dockerfile's
-- setcap step — a check that can't get a raw/ping socket at all (e.g. a
-- hardened deployment with cap_drop: [ALL]) fails with an explicit error
-- rather than silently reporting "down".
ALTER TABLE uptime_probes DROP CONSTRAINT uptime_probes_type_check;
ALTER TABLE uptime_probes ADD CONSTRAINT uptime_probes_type_check
    CHECK (type = ANY (ARRAY['http'::text, 'tcp'::text, 'icmp'::text]));
