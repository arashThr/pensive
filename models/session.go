package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/arashthr/go-course/rand"
	"github.com/arashthr/go-course/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionTokenBytes = 32

type Session struct {
	ID        uint
	UserId    types.UserId
	TokenHash string
	// Token is only set when creating a new session
	// This is empty when we look up session in db
	Token string
}

type SessionService struct {
	Pool *pgxpool.Pool
}

func (ss *SessionService) Create(userId types.UserId) (*Session, error) {
	token, err := rand.String(SessionTokenBytes)
	if err != nil {
		return nil, fmt.Errorf("session token: %w", err)
	}
	session := Session{
		UserId:    userId,
		Token:     token,
		TokenHash: ss.hash(token),
	}
	row := ss.Pool.QueryRow(context.Background(), `
		INSERT INTO sessions (user_id, token_hash)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET token_hash = $2
		RETURNING id;`, userId, session.TokenHash)
	err = row.Scan(&session.ID)
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return &session, nil
}

func (ss *SessionService) User(token string) (*User, error) {
	tokenHash := ss.hash(token)
	var user User

	row := ss.Pool.QueryRow(context.Background(), `
		SELECT users.id, email, password_hash, subscription_status
		FROM users
		JOIN sessions ON users.id = sessions.user_id
		WHERE sessions.token_hash = $1`, tokenHash)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.SubscriptionStatus)
	if err != nil {
		return nil, fmt.Errorf("session user: %w", err)
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

func (ss *SessionService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
