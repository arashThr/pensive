package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/arashthr/go-course/internal/rand"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionTokenBytes = 32

type Session struct {
	ID         uint
	UserId     types.UserId
	TokenHash  string
	IPAddress  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time
	// Token is only set when creating a new session
	// This is empty when we look up session in db
	Token string
}

type SessionService struct {
	Pool *pgxpool.Pool
}

func (ss *SessionService) Create(userId types.UserId, ipAddress string) (*Session, error) {
	err := ss.CleanupExpiredSessions()
	if err != nil {
		return nil, fmt.Errorf("cleanup expired sessions: %w", err)
	}

	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("session token: %w", err)
	}
	now := time.Now()
	expiresAt := now.Add(180 * 24 * time.Hour) // 6 months

	session := Session{
		UserId:     userId,
		Token:      token,
		TokenHash:  ss.hash(token),
		IPAddress:  ipAddress,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		LastUsedAt: now,
	}
	row := ss.Pool.QueryRow(context.Background(), `
		INSERT INTO sessions (user_id, token_hash, ip_address, expires_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, last_used_at;`, userId, session.TokenHash, ipAddress, expiresAt, now)
	err = row.Scan(&session.ID, &session.CreatedAt, &session.LastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return &session, nil
}

func (ss *SessionService) User(token string) (*User, error) {
	tokenHash := ss.hash(token)
	var user User

	row := ss.Pool.QueryRow(context.Background(), `
		SELECT users.id, email, password_hash, subscription_status, email_verified, oauth_email
		FROM users
		JOIN sessions ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1 AND sessions.expires_at > NOW()`, tokenHash)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.SubscriptionStatus, &user.EmailVerified, &user.OAuthEmail)
	if err != nil {
		return nil, fmt.Errorf("session user: %w", err)
	}

	// Update last_used_at when retrieving user
	_, err = ss.Pool.Exec(context.Background(), `
		UPDATE sessions 
		SET last_used_at = NOW() 
		WHERE token_hash = $1 AND expires_at > NOW()`, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("session update last_used_at: %w", err)
	}

	return &user, nil
}

func (ss *SessionService) Delete(token string) error {
	tokenHash := ss.hash(token)
	ex, err := ss.Pool.Exec(context.Background(), `
		DELETE FROM sessions WHERE token_hash = $1;`, tokenHash)
	if err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	fmt.Printf("ex: %+v\n", ex)
	return nil
}

// CleanupExpiredSessions removes all expired sessions from the database
func (ss *SessionService) CleanupExpiredSessions() error {
	result, err := ss.Pool.Exec(context.Background(), `
		DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return fmt.Errorf("cleanup expired sessions: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("Cleaned up %d expired sessions\n", rowsAffected)
	}

	return nil
}

func (ss *SessionService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
