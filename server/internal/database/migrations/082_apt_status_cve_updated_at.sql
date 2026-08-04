-- Tracks when apt_status.security_updates/cve_list were last actually
-- refreshed by the CVE-enriched pass (UpsertAptStatus), separately from
-- updated_at (bumped by the fast pending-packages-only path too,
-- UpsertAptPendingPackages) — lets the frontend tell "the package count is
-- fresh" apart from "the CVE detail is fresh".
ALTER TABLE apt_status ADD COLUMN cve_updated_at timestamp with time zone;
