CREATE TABLE IF NOT EXISTS stripe_customers (
    stripe_customer_id TEXT NOT NULL UNIQUE PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE subscriptions (
    id TEXT UNIQUE NOT NULL,
    user_id INTEGER NOT NULL,
    stripe_customer_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL, -- 'active', 'past_due', 'canceled'
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- -- Track subscription history for auditing and analytics
-- CREATE TABLE subscription_history (
--     id SERIAL PRIMARY KEY,
--     user_id INTEGER NOT NULL,
--     stripe_subscription_id TEXT NOT NULL,
--     tier TEXT NOT NULL,
--     status TEXT NOT NULL,
--     started_at TIMESTAMPTZ NOT NULL,
--     ended_at TIMESTAMPTZ NOT NULL,
--     FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
-- );

-- -- Track payment attempts
-- CREATE TABLE payment_history (
--     id SERIAL PRIMARY KEY,
--     user_id INTEGER NOT NULL FOREIGN KEY REFERENCES users(id),
--     stripe_subscription_id TEXT NOT NULL FOREIGN KEY REFERENCES subscriptions(stripe_subscription_id),
--     stripe_invoice_id TEXT NOT NULL UNIQUE,
--     amount_paid INTEGER NOT NULL,
--     currency TEXT NOT NULL DEFAULT 'usd',
--     status TEXT NOT NULL,
--     payment_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
-- );

-- Trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Indexes
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_stripe_customer_id ON subscriptions(stripe_customer_id);
-- CREATE INDEX idx_subscription_history_user_id ON subscription_history(user_id);
-- CREATE INDEX idx_payment_history_user_id ON payment_history(user_id);
