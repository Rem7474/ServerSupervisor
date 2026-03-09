-- Persist manual IP unblocks so they survive server restarts.
-- When an admin unblocks an IP, we record the timestamp here.
-- isIPBlocked only counts login failures that occurred AFTER the last unblock.

CREATE TABLE IF NOT EXISTS ip_block_overrides (
    ip_address  VARCHAR(45) PRIMARY KEY,
    unblocked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    unblocked_by VARCHAR(255) NOT NULL DEFAULT ''
);
