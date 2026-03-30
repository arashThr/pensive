CREATE TABLE password_resets (
    id         SERIAL PRIMARY KEY,
    user_id    INT UNIQUE NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
