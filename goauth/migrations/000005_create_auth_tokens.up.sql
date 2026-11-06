-- One-time tokens for passwordless auth and email verification.
CREATE TABLE auth_tokens (
    id         SERIAL PRIMARY KEY,
    email      TEXT NOT NULL,
    token_hash TEXT UNIQUE NOT NULL,
    token_type TEXT NOT NULL, -- 'signup' | 'signin' | 'email_verification'
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens (token_hash);
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens (expires_at);
