package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/arashthr/goauth/rand"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ApiTokenBytes = 32
	MaxApiTokens  = 5
)

// ApiToken represents a long-lived API token (for extensions, bots, etc.).
type ApiToken struct {
	ID          int
	UserID      UserID
	TokenHash   string
	TokenSource string // e.g. "extension", "manual", "telegram"
	CreatedAt   time.Time
	LastUsedAt  *time.Time
}

// GeneratedApiToken wraps ApiToken and includes the plaintext token.
type GeneratedApiToken struct {
	ApiToken
	Token string
}

// TokenRepo handles API tokens.
type TokenRepo struct {
	Pool *pgxpool.Pool
}

// Create generates and stores a new API token for the user.
// If the user already has MaxApiTokens, the oldest one is deleted first.
func (r *TokenRepo) Create(userID UserID, source string) (*GeneratedApiToken, error) {
	token, err := rand.String(ApiTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("api token – generate: %w", err)
	}

	var count int
	if err = r.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM api_tokens WHERE user_id = $1`, userID,
	).Scan(&count); err != nil {
		return nil, fmt.Errorf("api token – count: %w", err)
	}

	if count >= MaxApiTokens {
		_, err = r.Pool.Exec(context.Background(),
			`DELETE FROM api_tokens
			 WHERE id = (SELECT id FROM api_tokens WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1)`,
			userID)
		if err != nil {
			return nil, fmt.Errorf("api token – delete oldest: %w", err)
		}
	}

	at := GeneratedApiToken{
		ApiToken: ApiToken{
			UserID:      userID,
			TokenHash:   hashApiToken(token),
			TokenSource: source,
		},
		Token: token,
	}

	err = r.Pool.QueryRow(context.Background(),
		`INSERT INTO api_tokens (user_id, token_hash, token_source)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at`,
		at.UserID, at.TokenHash, at.TokenSource,
	).Scan(&at.ID, &at.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("api token – insert: %w", err)
	}
	return &at, nil
}

// Get returns all tokens for a user, ordered by newest first.
func (r *TokenRepo) Get(userID UserID) ([]ApiToken, error) {
	rows, err := r.Pool.Query(context.Background(),
		`SELECT id, user_id, token_hash, token_source, created_at, last_used_at
		 FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("api token get: %w", err)
	}
	defer rows.Close()
	tokens, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ApiToken])
	if err != nil {
		return nil, fmt.Errorf("api token collect: %w", err)
	}
	return tokens, nil
}

// User returns the user associated with the given plaintext API token.
func (r *TokenRepo) User(token string) (*User, error) {
	th := hashApiToken(token)
	rows, err := r.Pool.Query(context.Background(),
		`SELECT u.id, u.email, u.password_hash, u.oauth_provider, u.oauth_id, u.oauth_email,
		        u.email_verified, u.email_verified_at
		 FROM users u
		 JOIN api_tokens t ON u.id = t.user_id
		 WHERE t.token_hash = $1`, th)
	if err != nil {
		return nil, fmt.Errorf("api token user: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByPos[User])
	if err != nil {
		return nil, fmt.Errorf("api token collect user: %w", err)
	}

	_, err = r.Pool.Exec(context.Background(),
		`UPDATE api_tokens SET last_used_at = $1 WHERE token_hash = $2`,
		time.Now(), th)
	if err != nil {
		return nil, fmt.Errorf("api token update last_used_at: %w", err)
	}
	return u, nil
}

// Delete removes a token by ID for a specific user.
func (r *TokenRepo) Delete(userID UserID, tokenID string) error {
	_, err := r.Pool.Exec(context.Background(),
		`DELETE FROM api_tokens WHERE user_id = $1 AND id = $2`, userID, tokenID)
	if err != nil {
		return fmt.Errorf("api token delete: %w", err)
	}
	return nil
}

// DeleteByToken removes a token by its plaintext value.
func (r *TokenRepo) DeleteByToken(token string) error {
	_, err := r.Pool.Exec(context.Background(),
		`DELETE FROM api_tokens WHERE token_hash = $1`, hashApiToken(token))
	if err != nil {
		return fmt.Errorf("api token delete by token: %w", err)
	}
	return nil
}

func hashApiToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}
