-- WebAuthn (passkeys / security keys) as an additional MFA factor alongside TOTP.
-- The full go-webauthn Credential record (public key, sign counter, flags,
-- attestation, transports) is stored as one JSONB blob per the library's own
-- storage guidance, rather than split across columns — it's an opaque,
-- library-owned record we never query into, only round-trip.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    credential_data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user_id ON webauthn_credentials (user_id);
