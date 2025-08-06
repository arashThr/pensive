CREATE TABLE auth_tokens (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL,
    token_hash TEXT UNIQUE NOT NULL,
    token_type TEXT NOT NULL, -- 'signup' or 'signin'
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fast lookups by token hash
CREATE INDEX idx_auth_tokens_token_hash ON auth_tokens(token_hash);

-- Index for cleanup of expired tokens
CREATE INDEX idx_auth_tokens_expires_at ON auth_tokens(expires_at);