package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/rand"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultAuthTokenDuration = time.Minute * 15 // 15 minutes for magic links
)

type AuthTokenType string

const (
	AuthTokenTypeSignup AuthTokenType = "signup"
	AuthTokenTypeSignin AuthTokenType = "signin"
)

type AuthToken struct {
	ID        int
	Email     string
	// Token is only set when an AuthToken is being created.
	Token     string
	TokenHash string
	TokenType AuthTokenType
	ExpiresAt time.Time
	CreatedAt time.Time
}

type AuthTokenService struct {
	Pool     *pgxpool.Pool
	Duration time.Duration
	Now      func() time.Time
}

func NewAuthTokenService(pool *pgxpool.Pool) *AuthTokenService {
	return &AuthTokenService{
		Pool:     pool,
		Duration: DefaultAuthTokenDuration,
		Now:      time.Now,
	}
}

func (service *AuthTokenService) Create(email string, tokenType AuthTokenType) (*AuthToken, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("auth token generation: %w", err)
	}

	authToken := AuthToken{
		Email:     email,
		Token:     token,
		TokenHash: service.hash(token),
		TokenType: tokenType,
		ExpiresAt: service.Now().Add(service.Duration),
	}

	row := service.Pool.QueryRow(context.Background(),
		`INSERT INTO auth_tokens (email, token_hash, token_type, expires_at)
		VALUES($1, $2, $3, $4)
		RETURNING id, created_at;`, authToken.Email, authToken.TokenHash, authToken.TokenType, authToken.ExpiresAt)
	
	err = row.Scan(&authToken.ID, &authToken.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting auth token in db: %w", err)
	}

	return &authToken, nil
}

func (service *AuthTokenService) Consume(token string) (*AuthToken, error) {
	tokenHash := service.hash(token)
	var authToken AuthToken

	row := service.Pool.QueryRow(context.Background(),
		`SELECT id, email, token_type, expires_at, created_at
		FROM auth_tokens
		WHERE token_hash = $1;`, tokenHash)
	
	err := row.Scan(&authToken.ID, &authToken.Email, &authToken.TokenType, &authToken.ExpiresAt, &authToken.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetch auth token: %w", err)
	}

	if service.Now().After(authToken.ExpiresAt) {
		// Clean up expired token
		service.delete(authToken.ID)
		return nil, fmt.Errorf("expired token: %v", token)
	}

	err = service.delete(authToken.ID)
	if err != nil {
		return nil, fmt.Errorf("delete auth token: %w", err)
	}

	authToken.Token = token // Set the original token for reference
	return &authToken, nil
}

func (service *AuthTokenService) CleanupExpired() error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM auth_tokens WHERE expires_at < NOW();`)
	if err != nil {
		return fmt.Errorf("cleanup expired auth tokens: %w", err)
	}
	return nil
}

func (service *AuthTokenService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}

func (service *AuthTokenService) delete(id int) error {
	_, err := service.Pool.Exec(context.Background(),
		`DELETE FROM auth_tokens WHERE id = $1;`, id)
	if err != nil {
		return fmt.Errorf("delete auth token: %w", err)
	}
	return nil
}