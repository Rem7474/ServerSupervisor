-- Stores Nginx Proxy Manager instances and their imported proxy hosts.
-- Each connection authenticates with identity+secret to the NPM REST API.
-- Proxy hosts are imported manually (user selects via checkbox modal); the
-- background poller only refreshes last_seen_at/npm_enabled for already-imported hosts.
CREATE TABLE npm_connections (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT         NOT NULL,
    api_url          TEXT         NOT NULL,
    identity         TEXT         NOT NULL,
    secret           TEXT         NOT NULL,
    host_id          TEXT         REFERENCES hosts(id) ON DELETE SET NULL,
    enabled          BOOLEAN      NOT NULL DEFAULT true,
    poll_interval_sec INT         NOT NULL DEFAULT 3600,
    last_error       TEXT         NOT NULL DEFAULT '',
    last_error_at    TIMESTAMPTZ,
    last_success_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE npm_proxy_hosts (
    id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id      UUID         NOT NULL REFERENCES npm_connections(id) ON DELETE CASCADE,
    npm_id             INT          NOT NULL,
    domain_names       TEXT[]       NOT NULL DEFAULT '{}',
    forward_host       TEXT         NOT NULL DEFAULT '',
    forward_port       INT          NOT NULL DEFAULT 80,
    ssl_enabled        BOOLEAN      NOT NULL DEFAULT false,
    npm_enabled        BOOLEAN      NOT NULL DEFAULT true,
    uptime_probe_id    UUID         REFERENCES uptime_probes(id) ON DELETE SET NULL,
    ssl_certificate_id UUID         REFERENCES ssl_certificates(id) ON DELETE SET NULL,
    last_seen_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, npm_id)
);

CREATE INDEX idx_npm_proxy_hosts_connection ON npm_proxy_hosts(connection_id);
