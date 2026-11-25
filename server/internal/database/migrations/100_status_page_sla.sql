-- Availability objectives (SLO) per uptime probe, a long-lived daily rollup of
-- probe results, and the opt-in public status page.

-- Availability objective in percent (99.9 = "three nines"). Every probe gets
-- one so the SLA view has something to measure against from day one.
ALTER TABLE uptime_probes
    ADD COLUMN IF NOT EXISTS slo_target NUMERIC(6,3) NOT NULL DEFAULT 99.9
        CHECK (slo_target > 0 AND slo_target <= 100);

-- Raw uptime_probe_results rows are purged after 30 days. This table keeps one
-- row per probe and UTC day (check counts only, no error text) for a year, so
-- 90-day SLA windows and the status page's 90-day bars outlive the raw rows.
CREATE TABLE IF NOT EXISTS uptime_probe_daily (
    probe_id     UUID    NOT NULL REFERENCES uptime_probes(id) ON DELETE CASCADE,
    day          DATE    NOT NULL,
    total_checks INTEGER NOT NULL DEFAULT 0,
    up_checks    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (probe_id, day)
);

-- Single-row configuration of the public status page. Disabled by default.
CREATE TABLE IF NOT EXISTS status_page_config (
    id          BOOLEAN     PRIMARY KEY DEFAULT TRUE CHECK (id),
    enabled     BOOLEAN     NOT NULL DEFAULT FALSE,
    title       TEXT        NOT NULL DEFAULT '',
    description TEXT        NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO status_page_config (id) VALUES (TRUE) ON CONFLICT (id) DO NOTHING;

-- Probes an admin explicitly published, with the name shown to the public.
-- Nothing about the probe itself (target, type, errors) is ever exposed.
CREATE TABLE IF NOT EXISTS status_page_services (
    probe_id    UUID    PRIMARY KEY REFERENCES uptime_probes(id) ON DELETE CASCADE,
    public_name TEXT    NOT NULL CHECK (length(btrim(public_name)) BETWEEN 1 AND 80),
    position    INTEGER NOT NULL DEFAULT 0
);
