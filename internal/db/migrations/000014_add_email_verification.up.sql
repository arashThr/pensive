BEGIN;

-- Add email verification fields to users table
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMP;

-- Create index for email verification lookups
CREATE INDEX idx_users_email_verified ON users(email_verified);

-- Set existing OAuth users (who don't have passwords) as verified
-- since they've already gone through OAuth verification
UPDATE users SET email_verified = true, email_verified_at = NOW() 
WHERE oauth_provider IS NOT NULL AND oauth_id IS NOT NULL;

-- Set existing passwordless users (who have no password hash) as verified
-- since they've already verified their email through the magic link
UPDATE users SET email_verified = true, email_verified_at = NOW() 
WHERE password_hash IS NULL AND oauth_provider IS NULL;

COMMIT;