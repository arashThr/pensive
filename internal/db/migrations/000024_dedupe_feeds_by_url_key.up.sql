-- One feed per normalized URL, so http/https, "www." and trailing-slash
-- variants of the same feed are fetched and stored once. Keep this
-- expression in sync with models.FeedURLKey.
ALTER TABLE feeds ADD COLUMN url_key TEXT;
UPDATE feeds SET url_key = regexp_replace(
  regexp_replace(regexp_replace(lower(url), '#.*$', ''), '^[a-z][a-z0-9+.-]*://(www\.)?', ''),
  '/+(\?|$)', '\1');

-- Merge existing duplicates into the oldest feed for each key.
CREATE TEMP TABLE feed_dups ON COMMIT DROP AS
  SELECT f.id AS dup_id, k.keep_id
  FROM feeds f
  JOIN (SELECT url_key, MIN(id) AS keep_id FROM feeds GROUP BY url_key) k USING (url_key)
  WHERE f.id <> k.keep_id;

-- A user who followed several variants keeps one subscription; it stays
-- active if any variant was active.
INSERT INTO feed_subscriptions (user_id, feed_id, title, unsubscribed_at, created_at)
SELECT DISTINCT ON (s.user_id, d.keep_id) s.user_id, d.keep_id, s.title, s.unsubscribed_at, s.created_at
FROM feed_subscriptions s JOIN feed_dups d ON s.feed_id = d.dup_id
ORDER BY s.user_id, d.keep_id, s.unsubscribed_at DESC NULLS FIRST
ON CONFLICT (user_id, feed_id) DO UPDATE SET unsubscribed_at =
  CASE WHEN feed_subscriptions.unsubscribed_at IS NULL OR EXCLUDED.unsubscribed_at IS NULL THEN NULL
       ELSE GREATEST(feed_subscriptions.unsubscribed_at, EXCLUDED.unsubscribed_at) END;

INSERT INTO feed_entries (feed_id, guid, url, canonical_url, title, author, content, content_html, published_at, created_at)
SELECT d.keep_id, e.guid, e.url, e.canonical_url, e.title, e.author, e.content, e.content_html, e.published_at, e.created_at
FROM feed_entries e JOIN feed_dups d ON e.feed_id = d.dup_id
ON CONFLICT (feed_id, guid) DO NOTHING;

-- Subscriptions and entries of the duplicates go with them (ON DELETE CASCADE).
DELETE FROM feeds WHERE id IN (SELECT dup_id FROM feed_dups);

ALTER TABLE feeds ALTER COLUMN url_key SET NOT NULL;
CREATE UNIQUE INDEX feeds_url_key_idx ON feeds (url_key);
