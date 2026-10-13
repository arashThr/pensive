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

const DefaultResetDuration = time.Hour

// PasswordReset is a short-lived token used to reset a user's password.
type PasswordReset struct {
	ID        int
	UserID    UserID
	TokenHash string
	ExpiresAt time.Time

	// Token is only populated immediately after creation.
	Token string `db:"-"`
}

// PasswordResetRepo manages password reset tokens.
type PasswordResetRepo struct {
	Pool     *pgxpool.Pool
	Duration time.Duration
	Now      func() time.Time
}

// NewPasswordResetRepo creates a PasswordResetRepo with default expiration.
func NewPasswordResetRepo(pool *pgxpool.Pool) *PasswordResetRepo {
	return &PasswordResetRepo{Pool: pool, Duration: DefaultResetDuration, Now: time.Now}
}

// Create generates a reset token for the given email address.
// If the user already has a token it is replaced (one active token per user).
func (r *PasswordResetRepo) Create(email string) (*PasswordReset, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var userID UserID
	err := r.Pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE email = $1`, email,
	).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("password reset – find user: %w", err)
	}

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("password reset – generate token: %w", err)
	}

	pr := PasswordReset{
		UserID:    userID,
		Token:     token,
		TokenHash: r.hash(token),
		ExpiresAt: r.Now().Add(r.Duration),
	}

	err = r.Pool.QueryRow(context.Background(),
		`INSERT INTO password_resets (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET token_hash = $2, expires_at = $3
		 RETURNING id`,
		pr.UserID, pr.TokenHash, pr.ExpiresAt,
	).Scan(&pr.ID)
	if err != nil {
		return nil, fmt.Errorf("password reset – upsert: %w", err)
	}
	return &pr, nil
}

// Consume validates and consumes a reset token. Returns the associated user.
func (r *PasswordResetRepo) Consume(token string) (*User, error) {
	th := r.hash(token)

	var u User
	var pr PasswordReset
	err := r.Pool.QueryRow(context.Background(),
		`SELECT pr.id, pr.user_id, pr.expires_at, u.id, u.email, u.password_hash
		 FROM password_resets pr
		 JOIN users u ON u.id = pr.user_id
		 WHERE pr.token_hash = $1`, th,
	).Scan(&pr.ID, &pr.UserID, &pr.ExpiresAt, &u.ID, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("password reset – fetch: %w", err)
	}

	if r.Now().After(pr.ExpiresAt) {
		return nil, fmt.Errorf("password reset – expired token")
	}

	_, err = r.Pool.Exec(context.Background(),
		`DELETE FROM password_resets WHERE id = $1`, pr.ID)
	if err != nil {
		return nil, fmt.Errorf("password reset – delete: %w", err)
	}
	return &u, nil
}

func (r *PasswordResetRepo) hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}
