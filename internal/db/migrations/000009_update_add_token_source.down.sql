ALTER TABLE api_tokens DROP COLUMN token_source;

DROP INDEX idx_api_tokens_user_id;
