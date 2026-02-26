CREATE TYPE podcast_schedule_status AS ENUM (
    'pending',
    'processing',
    'sent',
    'failed',
    'timed_out'
);

CREATE TABLE podcast_schedules (
    id               SERIAL PRIMARY KEY,
    user_id          INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    next_publish_at  TIMESTAMPTZ NOT NULL,
    status           podcast_schedule_status NOT NULL DEFAULT 'pending',
    attempts         INTEGER NOT NULL DEFAULT 0,
    max_attempts     INTEGER NOT NULL DEFAULT 3,
    last_attempted_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podcast_schedules_user_id ON podcast_schedules (user_id);
CREATE INDEX idx_podcast_schedules_due ON podcast_schedules (next_publish_at, status);
