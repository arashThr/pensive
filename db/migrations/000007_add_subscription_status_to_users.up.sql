ALTER TABLE users
ADD COLUMN subscription_status TEXT NOT NULL DEFAULT 'free',
ADD COLUMN stripe_invoice_id TEXT;