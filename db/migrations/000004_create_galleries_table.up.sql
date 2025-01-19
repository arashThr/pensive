CREATE TABLE IF NOT EXISTS bookmarks (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    title TEXT,
    link TEXT
);