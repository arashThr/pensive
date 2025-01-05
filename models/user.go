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

func (us *UserService) Create(email, password string) (*User, error) {
	email = strings.ToLower(email)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	passwordHash := string(hashedBytes)

	row := us.pool.QueryRow(context.Background(), `
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

func (us *UserService) Update(user *User) error {
	return nil
}
