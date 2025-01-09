package models

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"

	"github.com/arashthr/go-course/rand"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionTokenBytes = 32

type Session struct {
	ID        uint
	UserId    uint
	TokenHash string
	// Token is only set when creating a new session
	// This is empty when we look up session in db
	Token string
}

type SessionService struct {
	Pool *pgxpool.Pool
}

func (ss *SessionService) Create(userId uint) (*Session, error) {
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
		UPDATE sessions
		SET token_hash = $1
		WHERE user_id = $2
		RETURNING id;`, session.TokenHash, userId)
	err = row.Scan(&session.ID)

	if err == sql.ErrNoRows {
		row = ss.Pool.QueryRow(context.Background(), `
			INSERT INTO sessions (user_id, token_hash)
			VALUES ($1, $2)
			RETURNING id;`, userId, session.TokenHash)
		err = row.Scan(&session.ID)
	}

	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}

	return &session, nil
}

func (ss *SessionService) User(token string) (*User, error) {
	return nil, nil
}

func (ss *SessionService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
