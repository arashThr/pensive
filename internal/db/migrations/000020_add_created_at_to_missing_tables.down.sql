ALTER TABLE library_contents
    DROP COLUMN IF EXISTS created_at;

ALTER TABLE password_resets
    DROP COLUMN IF EXISTS created_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS created_at;
