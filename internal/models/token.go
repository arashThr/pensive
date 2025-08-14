package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/arashthr/go-course/internal/logging"
	"github.com/arashthr/go-course/internal/rand"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenModel struct {
	Pool *pgxpool.Pool
}

const ApiTokenBytes = 32
const MaxTokens = 5

type ApiToken struct {
	ID          int
	UserId      types.UserId
	TokenHash   string
	TokenSource string
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

type GeneratedApiToken struct {
	ApiToken
	Token string
}

func (as *TokenModel) Create(userId types.UserId, source string) (*GeneratedApiToken, error) {
	token, err := rand.String(ApiTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("api token: %w", err)
	}

	// Check the limit on the number of tokens
	row := as.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM api_tokens WHERE user_id = $1`, userId)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("api token count: %w", err)
	}
	if count >= MaxTokens {
		logging.Logger.Warnw("api token limit reached. deleting old ones", "count", count, "userId", userId)
		// Delete the oldest token if the limit is reached
		_, err = as.Pool.Exec(context.Background(), `
			DELETE FROM api_tokens
			WHERE id = (
				SELECT id FROM api_tokens
				WHERE user_id = $1
				ORDER BY created_at ASC
				LIMIT 1
			)`, userId)
		if err != nil {
			return nil, fmt.Errorf("api token delete old: %w", err)
		}
		logging.Logger.Infow("old api token deleted", "userId", userId)
	}

	apiToken := GeneratedApiToken{
		ApiToken: ApiToken{
			UserId:      userId,
			TokenHash:   as.hash(token),
			TokenSource: source,
		},
		Token: token,
	}
	row = as.Pool.QueryRow(context.Background(), `
		INSERT INTO api_tokens (user_id, token_hash, token_source)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`, userId, apiToken.TokenHash, source)
	err = row.Scan(&apiToken.ID, &apiToken.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("api token create: %w", err)
	}
	return &apiToken, nil
}

func (as *TokenModel) Delete(userId types.UserId, tokenId string) error {
	_, err := as.Pool.Exec(context.Background(), `
		DELETE FROM api_tokens
		WHERE user_id = $1 AND id = $2`, userId, tokenId)
	if err != nil {
		return fmt.Errorf("api token delete: %w", err)
	}
	return nil
}

func (as *TokenModel) DeleteByToken(token string) error {
	tokenHash := as.hash(token)
	_, err := as.Pool.Exec(context.Background(), `
		DELETE FROM api_tokens
		WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("api token delete by token: %w", err)
	}
	return nil
}

func (as *TokenModel) Get(userId types.UserId) ([]ApiToken, error) {
	rows, err := as.Pool.Query(context.Background(), `
		SELECT id, user_id, token_hash, token_source, created_at, last_used_at
		FROM api_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`, userId)
	if err != nil {
		return nil, fmt.Errorf("api token rows get: %w", err)
	}
	defer rows.Close()
	validTokens, err := pgx.CollectRows(rows, pgx.RowToStructByName[ApiToken])
	if err != nil {
		return nil, fmt.Errorf("api token get: %w", err)
	}
	return validTokens, nil
}

func (as *TokenModel) User(token string) (*User, error) {
	tokenHash := as.hash(token)

	rows, err := as.Pool.Query(context.Background(), `
		SELECT users.*
		FROM users
		JOIN api_tokens ON users.id = api_tokens.user_id
		WHERE api_tokens.token_hash = $1`, tokenHash)

	if err != nil {
		return nil, fmt.Errorf("api user: %w", err)
	}

	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[User])

	if err != nil {
		return nil, fmt.Errorf("api collect one user: %w", err)
	}

	// Update token access time
	_, err = as.Pool.Exec(context.Background(), `
		UPDATE api_tokens
		SET last_used_at = $1
		WHERE token_hash = $2`, time.Now(), tokenHash)
	if err != nil {
		return nil, fmt.Errorf("api token update last used: %w", err)
	}
	return user, nil
}

func (as *TokenModel) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
