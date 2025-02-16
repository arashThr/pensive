package models

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uint
	Email        string
	PasswordHash string

	// Possible status values:
	// - 'none'        -> subscription was not created
	// - 'active'      -> subscription is active and paid
	// - 'past_due'    -> payment failed but still retrying
	// - 'unpaid'      -> payment failed and stopped retrying
	// - 'canceled'    -> subscription was canceled
	// - 'incomplete'  -> initial payment failed
	SubscriptionStatus string
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
		var pgErr interface {
			SQLState() string
		}
		if errors.As(err, &pgErr) {
			if pgErr.SQLState() == pgerrcode.UniqueViolation {
				return nil, ErrEmailTaken
			}
		}
		fmt.Printf("Type: %T\n", err)
		fmt.Printf("Value: %+v\n", err)
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

func (us *UserService) UpdatePassword(userId uint, password string) error {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	hashedPassword := string(hashedBytes)
	_, err = us.Pool.Exec(context.Background(), `UPDATE users SET password_hash = $1 WHERE id = $2`, hashedPassword, userId)
	if err != nil {
		return fmt.Errorf("update password in db: %w", err)
	}
	return nil
}
