-- Drop weekly_summaries table
DROP INDEX IF EXISTS idx_weekly_summaries_enabled_day;
DROP INDEX IF EXISTS idx_weekly_summaries_user_id;
DROP TABLE IF EXISTS weekly_summaries;
