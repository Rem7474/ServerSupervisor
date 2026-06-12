-- Adds per-proxy-host monitoring control columns to npm_proxy_hosts.
-- monitoring_enabled is the master switch; the sub-flags only take effect when it is true.
-- Both sub-flags default to true so existing imported hosts keep monitoring active after migration.
ALTER TABLE npm_proxy_hosts
    ADD COLUMN monitoring_enabled        BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN uptime_monitoring_enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN ssl_monitoring_enabled    BOOLEAN NOT NULL DEFAULT true;
