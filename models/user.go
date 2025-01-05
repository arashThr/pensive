package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uint
	Email        string
	PasswordHash string
}

type UserService struct {
	Pool *pgxpool.Pool
}

func normalizeEmail(email string) string {
	return strings.ToLower(email)
}

func (us *UserService) Create(email, password string) (*User, error) {
	email = normalizeEmail(email)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	passwordHash := string(hashedBytes)

	row := us.Pool.QueryRow(context.Background(), `
		INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id
	`, email, passwordHash)

	user := User{
		Email:        email,
		PasswordHash: passwordHash,
	}
	err = row.Scan(&user.ID)

	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

func (us *UserService) Authenticate(email, password string) (*User, error) {
	email = normalizeEmail(email)
	user := User{
		Email: email,
	}
	row := us.Pool.QueryRow(context.Background(), `
		SELECT id, password_hash FROM users WHERE email = $1
	`, email)
	err := row.Scan(&user.ID, &user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	return &user, nil
}
