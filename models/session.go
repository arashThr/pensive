package models

import "github.com/jackc/pgx/v5/pgxpool"

type Session struct {
	ID        uint
	UserId    int
	TokenHash string
	// Token is only set when creating a new session
	// This is empty when we look up session in db
	Token string
}

type SessionService struct {
	Pool *pgxpool.Pool
}

func (ss *SessionService) Create(userId uint) (*Session, error) {
	return nil, nil
}

func (ss *SessionService) User(token string) (*User, error) {
	return nil, nil
}
