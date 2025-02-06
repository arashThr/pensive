package models

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ApiService struct {
	Pool *pgxpool.Pool
}

func (as *ApiService) GenerateToken(userId uint) string {
	/*
		1. generate a random token
		2. saved the hashed version with user id in tokens table
		3. return token to the user
	*/
	return "token-token" + strconv.Itoa(int(userId))
}

func (as *ApiService) User(token string) (*User, error) {
	tokenHash := as.hash(token)
	var user User

	row := as.Pool.QueryRow(context.Background(), `
		SELECT users.id, email, password_hash
		FROM users
		JOIN api_tokens ON users.id = api_tokens.user_id
		WHERE api_tokens.token_hash = $1`, tokenHash)

	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("session user: %w", err)
	}
	return &user, nil

}

func (as *ApiService) hash(token string) string {
	tokenHash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(tokenHash[:])
}
