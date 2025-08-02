BEGIN;

-- Add OAuth provider support to users table
ALTER TABLE users ADD COLUMN oauth_provider TEXT;
ALTER TABLE users ADD COLUMN oauth_id TEXT;
ALTER TABLE users ADD COLUMN oauth_email TEXT;

-- Create unique constraint for OAuth provider + ID combination
ALTER TABLE users ADD CONSTRAINT unique_oauth_provider_id UNIQUE(oauth_provider, oauth_id);

-- Create index for OAuth lookups
CREATE INDEX idx_users_oauth_provider_id ON users(oauth_provider, oauth_id);
CREATE INDEX idx_users_oauth_email ON users(oauth_email);

-- Make password_hash nullable since OAuth users won't have passwords
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL; 

COMMIT;