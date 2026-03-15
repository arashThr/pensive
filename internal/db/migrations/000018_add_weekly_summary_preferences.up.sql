-- Create summaries_pref table for user podcast preferences
CREATE TABLE IF NOT EXISTS summaries_pref (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    day TEXT NOT NULL DEFAULT 'sunday',
    email BOOLEAN NOT NULL DEFAULT true,
    telegram BOOLEAN NOT NULL DEFAULT false,
    daily_enabled  BOOLEAN NOT NULL DEFAULT false,
    daily_hour INTEGER NOT NULL DEFAULT 8,
    daily_timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_summaries_pref_user_id ON summaries_pref(user_id);
CREATE INDEX idx_summaries_pref_enabled_day ON summaries_pref(enabled, day);
