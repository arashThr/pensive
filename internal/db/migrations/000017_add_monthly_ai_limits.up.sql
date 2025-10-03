-- Create table to track daily AI question usage
CREATE TABLE daily_ai_limits (
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day DATE NOT NULL, -- The date (e.g., 2025-01-15)
    question_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (user_id, day)
);

-- Index for efficient lookups
CREATE INDEX idx_daily_ai_limits_user_day ON daily_ai_limits(user_id, day);
