-- Stores the Telegram ↔ user linkage state.
CREATE TABLE telegram_auth (
    id               SERIAL PRIMARY KEY,
    user_id          INTEGER NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    auth_token       UUID NOT NULL,
    telegram_user_id BIGINT UNIQUE,
    token            TEXT,              -- API token string once linked
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_telegram_auth_user_id          ON telegram_auth (user_id);
CREATE INDEX idx_telegram_auth_telegram_user_id ON telegram_auth (telegram_user_id);
