-- Native Docker Compose update path for release trackers (Watchtower-like).
-- Adds a built-in `compose` deployment mode (pull + up -d) executed by a
-- dedicated, hardened agent module — no tasks.yaml required for the common case.
-- The server still never sends shell strings: it references a compose project
-- name that the agent only accepts if it exists in its locally discovered
-- inventory. Optional pre/post hooks reuse tasks.yaml (host-declared commands).

-- 1. Registry credentials for private-image manifest polling (GHCR, Harbor, ...).
--    Password stored in plaintext, mirroring proxmox_connections.token_secret —
--    protected by DB access control and never returned to the frontend.
CREATE TABLE IF NOT EXISTS registry_credentials (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL UNIQUE,
    registry_host VARCHAR(255) NOT NULL,        -- e.g. ghcr.io, registry.example.com
    username      VARCHAR(255) NOT NULL,
    password      TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. docker_containers.image_digest already exists (migration 020) and is
--    populated by the agent. Index it so drift detection (registry-available
--    vs running digest) can look up the deployed digest efficiently.
CREATE INDEX IF NOT EXISTS idx_docker_containers_image_digest
    ON docker_containers(image_digest) WHERE image_digest <> '';

-- 3. Release tracker compose-update configuration.
ALTER TABLE release_trackers
    -- 'custom' = legacy tasks.yaml dispatch (unchanged); 'compose' = native module
    ADD COLUMN IF NOT EXISTS update_action VARCHAR(20) NOT NULL DEFAULT 'custom',
    ADD COLUMN IF NOT EXISTS compose_project VARCHAR(100),
    ADD COLUMN IF NOT EXISTS compose_service VARCHAR(100),       -- NULL = whole project
    ADD COLUMN IF NOT EXISTS pre_update_task_id VARCHAR(64),     -- tasks.yaml id
    ADD COLUMN IF NOT EXISTS post_update_task_id VARCHAR(64),    -- tasks.yaml id
    ADD COLUMN IF NOT EXISTS cleanup_after_update BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS healthcheck_timeout_sec INT NOT NULL DEFAULT 0, -- 0 = disabled
    ADD COLUMN IF NOT EXISTS rollback_on_failure BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS registry_credentials_id UUID
        REFERENCES registry_credentials(id) ON DELETE SET NULL;

-- compose mode requires a target host and compose project.
ALTER TABLE release_trackers
    ADD CONSTRAINT release_trackers_compose_target_check
    CHECK (
        update_action <> 'compose'
        OR (host_id IS NOT NULL AND host_id <> '' AND compose_project IS NOT NULL AND compose_project <> '')
    );
