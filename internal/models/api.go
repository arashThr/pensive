package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/arashthr/go-course/internal/rand"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApiService struct {
	Pool *pgxpool.Pool
}

const ApiTokenBytes = 32

type ApiToken struct {
	ID        int
	UserId    types.UserId
	TokenHash string
	// Token is only set when creating a new session
	// This is empty when we look up token in db
	Token string
}

func (as *ApiService) Create(userId types.UserId) (*ApiToken, error) {
	token, err := rand.String(ApiTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("api token: %w", err)
	}
	apiToken := ApiToken{
		UserId:    userId,
		Token:     token,
		TokenHash: as.hash(token),
	}
	row := as.Pool.QueryRow(context.Background(), `
		INSERT INTO api_tokens (user_id, token_hash)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = $2
		RETURNING id;`, userId, apiToken.TokenHash)
	err = row.Scan(&apiToken.ID)
	if err != nil {
		return nil, fmt.Errorf("api token create: %w", err)
	}
	return &apiToken, nil
}

func (as *ApiService) Get(userId types.UserId) (*ApiToken, error) {
	var apiToken ApiToken
	row := as.Pool.QueryRow(context.Background(), `
		SELECT id, token_hash
		FROM api_tokens
		WHERE user_id = $1`, userId)
	err := row.Scan(&apiToken.ID, &apiToken.TokenHash)
	if err != nil {
		return nil, fmt.Errorf("api token get: %w", err)
	}
	log.Println("api token get spi tokn", apiToken)
	return &apiToken, nil
}

func (as *ApiService) User(token string) (*User, error) {
	tokenHash := as.hash(token)
	var user User

	row := as.Pool.QueryRow(context.Background(), `
		SELECT users.id, email, password_hash, subscription_status
		FROM users
		JOIN api_tokens ON users.id = api_tokens.user_id
		WHERE api_tokens.token_hash = $1`, tokenHash)

	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.SubscriptionStatus)
	if err != nil {
		return nil, fmt.Errorf("api user: %w", err)
	}
	return &user, nil

}

func (as *ApiService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
