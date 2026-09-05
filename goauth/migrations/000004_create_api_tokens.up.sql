CREATE TABLE api_tokens (
    id           SERIAL PRIMARY KEY,
    user_id      INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   TEXT UNIQUE NOT NULL,
    token_source TEXT NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_api_tokens_token_hash ON api_tokens (token_hash);
CREATE INDEX idx_api_tokens_user_id    ON api_tokens (user_id);
