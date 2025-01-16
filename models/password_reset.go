package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/arashthr/go-course/rand"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultResetDuration = time.Hour
)

type PasswordReset struct {
	ID     int
	UserID int
	// Token is only set when a PasswordReset is being created.
	Token     string
	TokenHash string
	ExpiresAt time.Time
}

type PasswordResetService struct {
	Pool     *pgxpool.Pool
	Duration time.Duration
	Now      func() time.Time
}

func (service *PasswordResetService) Create(email string) (*PasswordReset, error) {
	email = strings.ToLower(email)
	row := service.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE email = $1;`, email)
	var userId int
	err := row.Scan(&userId)
	if err != nil {
		return nil, fmt.Errorf("getting email: %w", err)
	}

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("password reset token: %w", err)
	}

	pwReset := PasswordReset{
		UserID:    userId,
		Token:     token,
		TokenHash: service.hash(token),
		ExpiresAt: service.Now().Add(time.Hour),
	}

	row = service.Pool.QueryRow(context.Background(),
		`INSERT INTO password_resets (user_id, token_hash, expires_at)
		VALUES($1, $2, $3)
		ON CONFLICT(user_id) DO UPDATE
		SET token_hash = $2, expires_at = $3
		RETURNING id;`, pwReset.UserID, pwReset.TokenHash, pwReset.ExpiresAt)
	err = row.Scan(&pwReset.ID)
	if err != nil {
		return nil, fmt.Errorf("updating password reset in db: %w", err)
	}
	return &pwReset, nil
}

func (service *PasswordResetService) Consume(token string) (*User, error) {
	return nil, fmt.Errorf("TODO: implement me")
}

func (ss *PasswordResetService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
