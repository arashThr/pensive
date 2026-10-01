-- Sanitized HTML of a post as the feed provides it, for the in-app reading view.
-- Empty for posts fetched before this column existed; they show plain text.
ALTER TABLE feed_entries ADD COLUMN content_html TEXT NOT NULL DEFAULT '';
