-- Create the immutable function
CREATE FUNCTION immutable_to_tsvector(text) RETURNS tsvector AS $$
SELECT to_tsvector('english', $1);
$$ LANGUAGE SQL IMMUTABLE;
CREATE TABLE users_bookmarks (
  bookmark_id TEXT PRIMARY KEY,
  user_id INT REFERENCES users(id),
  link TEXT NOT NULL,
  title TEXT NOT NULL,
  source TEXT NOT NULL,
  excerpt TEXT,
  image_url TEXT,
  article_lang TEXT,
  site_name TEXT,
  published_time TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT NOW()
  -- TODO: Add these columns
  -- source_id TEXT,
  -- tags TEXT[],
  -- summary TEXT,
);
CREATE TABLE bookmarks_contents (
  bookmark_id TEXT PRIMARY KEY REFERENCES users_bookmarks(bookmark_id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  excerpt TEXT NOT NULL,
  ai_markdown TEXT,
  search_vector tsvector GENERATED ALWAYS AS (immutable_to_tsvector(title || ' ' || excerpt || ' ' ||content)) STORED
);
CREATE INDEX search_vector_idx ON bookmarks_contents USING GIN(search_vector);