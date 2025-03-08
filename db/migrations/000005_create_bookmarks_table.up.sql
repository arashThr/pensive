CREATE TABLE IF NOT EXISTS bookmarks (
    bookmark_id TEXT PRIMARY KEY,
    user_id INT REFERENCES users(id),
    title TEXT,
    link TEXT
);