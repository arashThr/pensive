package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/arashthr/goauth/rand"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultAuthTokenDuration = 15 * time.Minute

// AuthTokenType categorises the purpose of a one-time token.
type AuthTokenType string

const (
	AuthTokenTypeSignup            AuthTokenType = "signup"
	AuthTokenTypeSignin            AuthTokenType = "signin"
	AuthTokenTypeEmailVerification AuthTokenType = "email_verification"
)

// AuthToken is a short-lived, one-time-use token sent to users via email.
type AuthToken struct {
	ID        int
	Email     string
	TokenHash string
	TokenType AuthTokenType
	ExpiresAt time.Time
	CreatedAt time.Time

	// Token is only populated immediately after creation.
	Token string `db:"-"`
}

// AuthTokenService manages one-time auth tokens.
type AuthTokenService struct {
	Pool     *pgxpool.Pool
	Duration time.Duration
	Now      func() time.Time
}

// NewAuthTokenService creates a new AuthTokenService with default duration.
func NewAuthTokenService(pool *pgxpool.Pool) *AuthTokenService {
	return &AuthTokenService{
		Pool:     pool,
		Duration: DefaultAuthTokenDuration,
		Now:      time.Now,
	}
}

// Create generates and stores a new one-time token for the given email.
func (s *AuthTokenService) Create(email string, tokenType AuthTokenType) (*AuthToken, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("auth token – generate: %w", err)
	}

	at := AuthToken{
		Email:     email,
		Token:     token,
		TokenHash: s.hash(token),
		TokenType: tokenType,
		ExpiresAt: s.Now().Add(s.Duration),
	}

	err = s.Pool.QueryRow(context.Background(),
		`INSERT INTO auth_tokens (email, token_hash, token_type, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		at.Email, at.TokenHash, at.TokenType, at.ExpiresAt,
	).Scan(&at.ID, &at.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth token – insert: %w", err)
	}
	return &at, nil
}

// Consume validates and consumes a token (one-time use). Returns the token record.
func (s *AuthTokenService) Consume(token string) (*AuthToken, error) {
	th := s.hash(token)
	var at AuthToken

	err := s.Pool.QueryRow(context.Background(),
		`SELECT id, email, token_type, expires_at, created_at
		 FROM auth_tokens WHERE token_hash = $1`, th,
	).Scan(&at.ID, &at.Email, &at.TokenType, &at.ExpiresAt, &at.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth token – fetch: %w", err)
	}

	if s.Now().After(at.ExpiresAt) {
		_ = s.delete(at.ID)
		return nil, fmt.Errorf("auth token – expired: %w", err)
	}

	if err = s.delete(at.ID); err != nil {
		return nil, fmt.Errorf("auth token – delete after consume: %w", err)
	}

	at.Token = token
	return &at, nil
}

// CleanupExpired removes all expired tokens.
func (s *AuthTokenService) CleanupExpired() error {
	_, err := s.Pool.Exec(context.Background(),
		`DELETE FROM auth_tokens WHERE expires_at < NOW()`)
	return err
}

func (s *AuthTokenService) hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}

func (s *AuthTokenService) delete(id int) error {
	_, err := s.Pool.Exec(context.Background(),
		`DELETE FROM auth_tokens WHERE id = $1`, id)
	return err
}
