CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT UNIQUE NOT NULL,
    subscription_status TEXT NOT NULL DEFAULT 'free',
    stripe_invoice_id TEXT                  
);
