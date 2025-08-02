BEGIN;

-- Revert OAuth provider support from users table
DROP INDEX IF EXISTS idx_users_oauth_email;
DROP INDEX IF EXISTS idx_users_oauth_provider_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS unique_oauth_provider_id;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_email;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_id;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_provider;

-- Make password_hash required again
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL; 

COMMIT;