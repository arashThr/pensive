-- RSS/Atom feeds. A feed is stored and fetched once, however many users follow it.
CREATE TABLE feeds (
  id BIGSERIAL PRIMARY KEY,
  url TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  site_url TEXT NOT NULL DEFAULT '',
  -- HTTP caching validators for conditional requests
  etag TEXT NOT NULL DEFAULT '',
  last_modified TEXT NOT NULL DEFAULT '',
  next_fetch_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_fetched_at TIMESTAMPTZ,
  error_count INT NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX feeds_next_fetch_at_idx ON feeds (next_fetch_at);

-- Who follows what. Unsubscribing sets unsubscribed_at instead of deleting, so
-- posts collected while subscribed stay searchable.
CREATE TABLE feed_subscriptions (
  user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  feed_id BIGINT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  title TEXT NOT NULL DEFAULT '',
  unsubscribed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, feed_id)
);
CREATE INDEX feed_subscriptions_feed_id_idx ON feed_subscriptions (feed_id);

-- Posts, kept indefinitely as a searchable archive. Content is plain text.
CREATE TABLE feed_entries (
  id BIGSERIAL PRIMARY KEY,
  feed_id BIGINT NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid TEXT NOT NULL,
  url TEXT NOT NULL,
  -- url as library_items.link stores it, to tell whether a post is already saved
  canonical_url TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', title), 'A') ||
    to_tsvector('english', content)
  ) STORED,
  UNIQUE (feed_id, guid)
);
CREATE INDEX feed_entries_search_idx ON feed_entries USING GIN (search_vector);
CREATE INDEX feed_entries_feed_published_idx ON feed_entries (feed_id, published_at DESC);
