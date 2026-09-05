package models

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TelegramRepo manages Telegram authentication state.
type TelegramRepo struct {
	Pool *pgxpool.Pool
}

// CreateAuthToken creates or replaces a short-lived UUID token used to link
// a Telegram account to a Pensive user (valid for 5 minutes).
func (r *TelegramRepo) CreateAuthToken(userID UserID) (string, error) {
	token := uuid.New().String()
	_, err := r.Pool.Exec(context.Background(),
		`INSERT INTO telegram_auth (user_id, auth_token, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id) DO UPDATE
		 SET auth_token = $2, telegram_user_id = NULL, updated_at = NOW()`,
		userID, token)
	if err != nil {
		return "", fmt.Errorf("telegram create auth token: %w", err)
	}
	return token, nil
}

// GetUserFromAuthToken resolves a token (must be < 5 minutes old) to a user ID.
func (r *TelegramRepo) GetUserFromAuthToken(token string) (UserID, error) {
	var userID UserID
	err := r.Pool.QueryRow(context.Background(),
		`SELECT user_id FROM telegram_auth
		 WHERE auth_token = $1 AND updated_at > NOW() - INTERVAL '5 minutes'`, token,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("telegram get user from auth token: %w", err)
	}
	return userID, nil
}

// SetChatID stores the Telegram chat ID and the API token string for a user.
func (r *TelegramRepo) SetChatID(userID UserID, chatID int64, apiToken string) error {
	_, err := r.Pool.Exec(context.Background(),
		`UPDATE telegram_auth
		 SET telegram_user_id = $2, token = $3, updated_at = NOW()
		 WHERE user_id = $1`,
		userID, chatID, apiToken)
	if err != nil {
		return fmt.Errorf("telegram set chat id: %w", err)
	}
	return nil
}

// GetAPIToken retrieves the stored API token string for a Telegram user ID.
func (r *TelegramRepo) GetAPIToken(telegramUserID int64) string {
	var token string
	_ = r.Pool.QueryRow(context.Background(),
		`SELECT token FROM telegram_auth WHERE telegram_user_id = $1`, telegramUserID,
	).Scan(&token)
	return token
}

// GetChatIDByUserID returns the Telegram chat ID for a given app user ID.
func (r *TelegramRepo) GetChatIDByUserID(userID UserID) (int64, error) {
	var chatID int64
	err := r.Pool.QueryRow(context.Background(),
		`SELECT telegram_user_id FROM telegram_auth
		 WHERE user_id = $1 AND telegram_user_id IS NOT NULL`, userID,
	).Scan(&chatID)
	if err != nil {
		return 0, fmt.Errorf("telegram get chat id: %w", err)
	}
	return chatID, nil
}
