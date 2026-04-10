package models

import (
	"context"
	"fmt"

	"github.com/arashthr/pensive/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TelegramRepo struct {
	Pool *pgxpool.Pool
}

func (t *TelegramRepo) CreateAuthToken(userId types.UserId) (string, error) {
	token := uuid.New().String()
	_, err := t.Pool.Exec(context.Background(), `
		INSERT INTO telegram_auth (user_id, auth_token, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET auth_token = $2, telegram_user_id = NULL, updated_at = NOW()`, userId, token)
	if err != nil {
		return "", fmt.Errorf("failed to create auth token: %w", err)
	}
	return token, nil
}

func (t *TelegramRepo) GetUserFromAuthToken(token string) (types.UserId, error) {
	var userId types.UserId
	err := t.Pool.QueryRow(context.Background(), `
		SELECT user_id
		FROM telegram_auth
		WHERE auth_token = $1
		AND updated_at > NOW() - INTERVAL '5 minutes'
	`, token).Scan(&userId)
	if err != nil {
		return 0, fmt.Errorf("failed to get auth token: %w", err)
	}
	return userId, nil
}

func (t *TelegramRepo) SetTokenForChatId(userId types.UserId, chatId int64, token *GeneratedApiToken) error {
	_, err := t.Pool.Exec(context.Background(), `
		UPDATE telegram_auth
		SET telegram_user_id = $2, token = $3, updated_at = NOW()
		WHERE user_id = $1`, userId, chatId, token.Token)
	if err != nil {
		return fmt.Errorf("failed to update chat id: %w", err)
	}
	return nil
}

func (t *TelegramRepo) GetToken(userId int64) string {
	var token string
	err := t.Pool.QueryRow(context.Background(), `
		SELECT token
		FROM telegram_auth
		WHERE telegram_user_id = $1
	`, userId).Scan(&token)
	if err != nil {
		return ""
	}
	return token
}

// GetChatIdByUserId returns the Telegram chat ID for a given Pensive user ID
func (t *TelegramRepo) GetChatIdByUserId(userId types.UserId) (int64, error) {
	var chatId int64
	err := t.Pool.QueryRow(context.Background(), `
		SELECT telegram_user_id
		FROM telegram_auth
		WHERE user_id = $1 AND telegram_user_id IS NOT NULL
	`, userId).Scan(&chatId)
	if err != nil {
		return 0, fmt.Errorf("get telegram chat id: %w", err)
	}
	return chatId, nil
}

// CreateConnectToken creates (or replaces) a short-lived token for a Telegram
// chat that has not yet been linked to a Pensive account. The token is used
// by the web flow initiated from the bot.
func (t *TelegramRepo) CreateConnectToken(chatId int64) (string, error) {
	token := uuid.New().String()
	_, err := t.Pool.Exec(context.Background(), `
		INSERT INTO telegram_connect_tokens (chat_id, token, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (chat_id) DO UPDATE SET token = $2, created_at = NOW()
	`, chatId, token)
	if err != nil {
		return "", fmt.Errorf("create connect token: %w", err)
	}
	return token, nil
}

// GetChatIdFromConnectToken looks up the chat_id for a given connect token.
// Tokens expire after 15 minutes.
func (t *TelegramRepo) GetChatIdFromConnectToken(token string) (int64, error) {
	var chatId int64
	err := t.Pool.QueryRow(context.Background(), `
		SELECT chat_id FROM telegram_connect_tokens
		WHERE token = $1 AND created_at > NOW() - INTERVAL '15 minutes'
	`, token).Scan(&chatId)
	if err != nil {
		return 0, fmt.Errorf("get chat id from connect token: %w", err)
	}
	return chatId, nil
}

// DeleteConnectToken removes a used (or stale) connect token.
func (t *TelegramRepo) DeleteConnectToken(token string) error {
	_, err := t.Pool.Exec(context.Background(), `
		DELETE FROM telegram_connect_tokens WHERE token = $1
	`, token)
	return err
}

// UpsertConnection links a Telegram chat to a Pensive user and stores the API
// token. Any previous link for the same chat_id on a different user is cleared
// first to respect the UNIQUE constraint.
func (t *TelegramRepo) UpsertConnection(userId types.UserId, chatId int64, apiToken string) error {
	ctx := context.Background()
	// Clear a previous chat_id association on a different user
	_, err := t.Pool.Exec(ctx, `
		UPDATE telegram_auth SET telegram_user_id = NULL
		WHERE telegram_user_id = $1 AND user_id != $2
	`, chatId, userId)
	if err != nil {
		return fmt.Errorf("clear stale chat id: %w", err)
	}

	_, err = t.Pool.Exec(ctx, `
		INSERT INTO telegram_auth (user_id, auth_token, telegram_user_id, token, updated_at)
		VALUES ($1, gen_random_uuid(), $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET telegram_user_id = $2, token = $3, updated_at = NOW()
	`, userId, chatId, apiToken)
	if err != nil {
		return fmt.Errorf("upsert telegram connection: %w", err)
	}
	return nil
}
