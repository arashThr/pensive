-- Create the immutable function
CREATE FUNCTION immutable_to_tsvector(text) RETURNS tsvector AS $$
  SELECT to_tsvector('english', $1);
$$ LANGUAGE SQL IMMUTABLE;

CREATE TABLE bookmarks (
    bookmark_id TEXT PRIMARY KEY,
    user_id INT REFERENCES users(id),
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,
    excerpt TEXT,
    image_url TEXT,
    article_lang TEXT,
    site_name TEXT,
    published_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    -- TODO: Add these columns
    -- summary TEXT,
    -- source_id TEXT,
    -- tags TEXT[],
    search_vector tsvector GENERATED ALWAYS AS (immutable_to_tsvector(title || ' ' || content)) STORED
);

CREATE INDEX search_vector_idx ON bookmarks USING GIN(search_vector);
