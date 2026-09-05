CREATE TABLE users (
    id             SERIAL PRIMARY KEY,
    email          TEXT UNIQUE NOT NULL,
    password_hash  TEXT,
    oauth_provider TEXT,
    oauth_id       TEXT,
    oauth_email    TEXT,
    email_verified    BOOLEAN NOT NULL DEFAULT false,
    email_verified_at TIMESTAMP,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_oauth UNIQUE (oauth_provider, oauth_id)
);

CREATE INDEX idx_users_email           ON users (email);
CREATE INDEX idx_users_oauth           ON users (oauth_provider, oauth_id);
