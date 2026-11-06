package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/arashthr/goauth/rand"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SessionTokenBytes  = 32
	SessionExpiration  = 180 * 24 * time.Hour // 6 months
)

// Session represents an authenticated browser session.
type Session struct {
	ID         uint
	UserID     UserID
	TokenHash  string
	IPAddress  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time

	// Token is only populated immediately after creation; never stored in the DB.
	Token string `db:"-"`
}

// SessionRepo handles all database operations for sessions.
type SessionRepo struct {
	Pool *pgxpool.Pool
}

// Create inserts a new session and returns it with the plaintext token set.
func (r *SessionRepo) Create(userID UserID, ipAddress string) (*Session, error) {
	if err := r.CleanupExpired(); err != nil {
		return nil, fmt.Errorf("session create – cleanup: %w", err)
	}

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("session create – token: %w", err)
	}

	now := time.Now()
	s := Session{
		UserID:     userID,
		Token:      token,
		TokenHash:  hashToken(token),
		IPAddress:  ipAddress,
		CreatedAt:  now,
		ExpiresAt:  now.Add(SessionExpiration),
		LastUsedAt: now,
	}

	err = r.Pool.QueryRow(context.Background(),
		`INSERT INTO sessions (user_id, token_hash, ip_address, expires_at, last_used_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, last_used_at`,
		s.UserID, s.TokenHash, s.IPAddress, s.ExpiresAt, s.LastUsedAt,
	).Scan(&s.ID, &s.CreatedAt, &s.LastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return &s, nil
}

// User looks up the user belonging to a valid (non-expired) session token.
// It also updates last_used_at.
func (r *SessionRepo) User(token string) (*User, error) {
	th := hashToken(token)
	var u User
	err := r.Pool.QueryRow(context.Background(),
		`SELECT u.id, u.email, u.password_hash, u.oauth_provider, u.oauth_id, u.oauth_email,
		        u.email_verified, u.email_verified_at
		 FROM users u
		 JOIN sessions s ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > NOW()`, th,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.OAuthProvider, &u.OAuthID, &u.OAuthEmail,
		&u.EmailVerified, &u.EmailVerifiedAt)
	if err != nil {
		return nil, fmt.Errorf("session user: %w", err)
	}

	_, err = r.Pool.Exec(context.Background(),
		`UPDATE sessions SET last_used_at = NOW() WHERE token_hash = $1 AND expires_at > NOW()`, th)
	if err != nil {
		return nil, fmt.Errorf("session update last_used_at: %w", err)
	}
	return &u, nil
}

// Delete removes a session by its plaintext token.
func (r *SessionRepo) Delete(token string) error {
	_, err := r.Pool.Exec(context.Background(),
		`DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	if err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

// CleanupExpired removes all expired sessions from the database.
func (r *SessionRepo) CleanupExpired() error {
	_, err := r.Pool.Exec(context.Background(),
		`DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}
	return nil
}

// hashToken returns the base64-encoded SHA-256 hash of a token.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}
