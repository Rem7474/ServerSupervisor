-- Captures the raw JSON body of each received git webhook delivery so an admin
-- can inspect exactly what a provider sent (debugging "why didn't this
-- trigger") instead of only the handful of fields already parsed out of it.
-- Plain TEXT (not JSONB): the stored value is truncated to a bounded size for
-- oversized deliveries, which can leave it invalid as JSON — a JSONB column
-- would reject that at INSERT time.
ALTER TABLE git_webhook_executions ADD COLUMN IF NOT EXISTS raw_payload TEXT;
