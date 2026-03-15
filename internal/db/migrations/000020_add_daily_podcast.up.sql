-- Add daily podcast preference columns to weekly_summaries
ALTER TABLE weekly_summaries
    ADD COLUMN IF NOT EXISTS daily_enabled  BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS daily_hour     INTEGER NOT NULL DEFAULT 8,
    ADD COLUMN IF NOT EXISTS daily_timezone TEXT    NOT NULL DEFAULT 'UTC';
