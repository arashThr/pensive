package models

import (
	"context"
	"fmt"

	"github.com/arashthr/pensive/internal/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TelegramService struct {
	Pool *pgxpool.Pool
}

func (t *TelegramService) CreateAuthToken(userId types.UserId) (string, error) {
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

func (t *TelegramService) GetUserFromAuthToken(token string) (types.UserId, error) {
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

func (t *TelegramService) SetTokenForChatId(userId types.UserId, chatId int64, token *GeneratedApiToken) error {
	_, err := t.Pool.Exec(context.Background(), `
		UPDATE telegram_auth
		SET telegram_user_id = $2, token = $3, updated_at = NOW()
		WHERE user_id = $1`, userId, chatId, token.Token)
	if err != nil {
		return fmt.Errorf("failed to update chat id: %w", err)
	}
	return nil
}

func (t *TelegramService) GetToken(userId int64) string {
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
