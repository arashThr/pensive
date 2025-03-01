ALTER TABLE users
DROP CONSTRAINT valid_status,
DROP COLUMN subscription_status,
DROP COLUMN stripe_invoice_id;