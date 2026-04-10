CREATE TABLE telegram_connect_tokens (
    id         SERIAL PRIMARY KEY,
    chat_id    BIGINT NOT NULL UNIQUE,
    token      UUID   NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX ON telegram_connect_tokens(token);
