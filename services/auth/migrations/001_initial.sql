CREATE SCHEMA IF NOT EXISTS garance_auth;

CREATE TABLE IF NOT EXISTS garance_auth.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    encrypted_password TEXT, -- NULL for OAuth-only / magic link users
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    role TEXT NOT NULL DEFAULT 'user',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    banned_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS garance_auth.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    refresh_token TEXT UNIQUE NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON garance_auth.sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON garance_auth.sessions(refresh_token) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS garance_auth.identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL, -- 'google', 'github', 'gitlab'
    provider_user_id TEXT NOT NULL,
    provider_data JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_identities_user_id ON garance_auth.identities(user_id);

CREATE TABLE IF NOT EXISTS garance_auth.verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES garance_auth.users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL, -- 'email_verification', 'password_reset', 'magic_link'
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_verification_tokens_token ON garance_auth.verification_tokens(token) WHERE used_at IS NULL;
