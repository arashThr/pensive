ALTER TABLE weekly_summaries
    DROP COLUMN IF EXISTS daily_enabled,
    DROP COLUMN IF EXISTS daily_hour,
    DROP COLUMN IF EXISTS daily_timezone;
