CREATE TABLE sessions (
    id           SERIAL PRIMARY KEY,
    user_id      INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   TEXT UNIQUE NOT NULL,
    ip_address   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_token_hash ON sessions (token_hash);
CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
