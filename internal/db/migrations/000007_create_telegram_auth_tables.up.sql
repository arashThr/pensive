CREATE TABLE telegram_auth (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    auth_token UUID NOT NULL,
    telegram_user_id BIGINT,
    token TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    UNIQUE(telegram_user_id)
);

CREATE INDEX ON telegram_auth(user_id);
CREATE INDEX ON telegram_auth(telegram_user_id);
