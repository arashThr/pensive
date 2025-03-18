package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 types.UserId
	Email              string
	PasswordHash       string
	SubscriptionStatus string
}

type UserService struct {
	Pool *pgxpool.Pool
}

func normalizeEmail(email string) string {
	return strings.ToLower(email)
}

// These are the states where user should have access
var ActiveStates = map[string]bool{
	"active":   true,
	"trialing": true,
}

// These states require attention but might still have access
var GracePeriodStates = map[string]bool{
	"past_due": true, // Configurable based on your business rules
}

// These states should definitely not have access
var InactiveStates = map[string]bool{
	"canceled":           true,
	"unpaid":             true,
	"incomplete":         true,
	"incomplete_expired": true,
	"free":               true,
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
				return nil, errors.ErrEmailTaken
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

func (us *UserService) UpdatePassword(userId types.UserId, password string) error {
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
