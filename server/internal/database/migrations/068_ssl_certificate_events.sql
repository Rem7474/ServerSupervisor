-- Tracks historical SSL certificate versions (renewals) per monitored endpoint.
-- A new row is inserted when a check detects a serial_number not yet seen for
-- that certificate, so the table records the full renewal timeline.
CREATE TABLE ssl_certificate_events (
    id             BIGSERIAL    PRIMARY KEY,
    certificate_id UUID         NOT NULL REFERENCES ssl_certificates(id) ON DELETE CASCADE,
    serial_number  TEXT         NOT NULL,
    valid_from     TIMESTAMPTZ,
    valid_to       TIMESTAMPTZ,
    issuer         TEXT         NOT NULL DEFAULT '',
    subject        TEXT         NOT NULL DEFAULT '',
    detected_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (certificate_id, serial_number)
);

CREATE INDEX idx_ssl_certificate_events_cert ON ssl_certificate_events(certificate_id, detected_at DESC);
