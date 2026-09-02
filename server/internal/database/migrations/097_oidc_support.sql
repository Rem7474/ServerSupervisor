-- =====================================================================
-- Migration 097: Support OpenID Connect (OIDC / SSO)
-- =====================================================================

-- 1. Add OIDC columns to users table and allow nullable password_hash for OIDC-only users
ALTER TABLE public.users
    ADD COLUMN IF NOT EXISTS auth_provider character varying(32) DEFAULT 'local' NOT NULL,
    ADD COLUMN IF NOT EXISTS oidc_sub text,
    ADD COLUMN IF NOT EXISTS email character varying(255);

ALTER TABLE public.users
    ALTER COLUMN password_hash DROP NOT NULL;

-- 2. Unique index for OIDC subject
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_oidc_sub
    ON public.users (oidc_sub)
    WHERE oidc_sub IS NOT NULL;

-- 3. Index for user lookup by email
CREATE INDEX IF NOT EXISTS idx_users_email
    ON public.users (email)
    WHERE email IS NOT NULL;

-- 4. Temporary OIDC authentication states (PKCE verifiers, nonces, redirection targets)
CREATE TABLE IF NOT EXISTS public.oidc_auth_states (
    state_id character varying(64) PRIMARY KEY,
    nonce character varying(64) NOT NULL,
    code_verifier text NOT NULL,
    redirect_url text DEFAULT '/'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oidc_auth_states_expires
    ON public.oidc_auth_states (expires_at);
