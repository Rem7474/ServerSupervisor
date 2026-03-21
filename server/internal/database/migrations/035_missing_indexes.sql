-- Migration 035: indexes manquants identifiés à l'audit
-- Tous les statements sont idempotents (IF NOT EXISTS).

-- remote_commands.audit_log_id : utilisé par CleanupStalledCommands et UpdateAuditLogStatus
-- Sans cet index, chaque suppression d'audit_logs provoque un full scan de remote_commands.
CREATE INDEX IF NOT EXISTS idx_commands_audit_log_id
    ON remote_commands(audit_log_id)
    WHERE audit_log_id IS NOT NULL;

-- release_tracker_tag_digests: accès par (tracker_id, created_at) pour le cleanup keepPerTracker
CREATE INDEX IF NOT EXISTS idx_rttd_tracker_created
    ON release_tracker_tag_digests(tracker_id, created_at DESC);
