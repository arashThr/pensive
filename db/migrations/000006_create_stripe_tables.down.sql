DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS subscription_events;
DROP TRIGGER IF EXISTS update_subscriptions_updated_at ON subscriptions;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS stripe_customers;