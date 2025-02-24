ALTER TABLE users
ADD COLUMN subscription_status TEXT NOT NULL DEFAULT 'free',
ADD CONSTRAINT valid_status CHECK (subscription_status IN ('free', 'trialing', 'active', 'incomplete', 'incomplete_expired', 'past_due', 'canceled', 'unpaid', 'paused'));