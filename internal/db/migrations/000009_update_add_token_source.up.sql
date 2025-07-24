-- Token source will be used to identify the source of the token, like extension or Telegram
ALTER TABLE api_tokens ADD COLUMN token_source TEXT NOT NULL DEFAULT 'manual';

-- Add index on user_id
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
