package models

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/types"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type SubscriptionStatus string

const (
	SubscriptionStatusFree        SubscriptionStatus = "free"
	SubscriptionStatusPremium     SubscriptionStatus = "premium"
	SubscriptionStatusFreePremium SubscriptionStatus = "free-premium"
)

type User struct {
	ID                 types.UserId
	Email              string
	PasswordHash       *string // Made nullable for OAuth users
	SubscriptionStatus SubscriptionStatus
	StripeInvoiceId    *string
	OAuthProvider      *string
	OAuthID            *string
	OAuthEmail         *string
}

type UserModel struct {
	Pool *pgxpool.Pool
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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

func (u *User) IsSubscriptionPremium() bool {
	return u.SubscriptionStatus == SubscriptionStatusPremium ||
		u.SubscriptionStatus == SubscriptionStatusFreePremium
}

func (us *UserModel) Create(email, password string) (*User, error) {
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
		PasswordHash: &passwordHash,
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

func (us *UserModel) Get(userId types.UserId) (*User, error) {
	rows, err := us.Pool.Query(context.Background(), `
		SELECT * FROM users WHERE id = $1;`, userId)

	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("collect one user: %w", err)
	}
	return &user, nil
}

func (us *UserModel) Authenticate(email, password string) (*User, error) {
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

	// Check if user has a password (not OAuth-only user)
	if user.PasswordHash == nil {
		return nil, fmt.Errorf("authenticate: user has no password set")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	return &user, nil
}

func (us *UserModel) UpdatePassword(userId types.UserId, password string) error {
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

// OAuth methods
func (us *UserModel) GetByOAuth(provider, oauthID string) (*User, error) {
	rows, err := us.Pool.Query(context.Background(), `
		SELECT *
		FROM users
		WHERE oauth_provider = $1 AND oauth_id = $2
	`, provider, oauthID)

	if err != nil {
		return nil, fmt.Errorf("get user by oauth: %w", err)
	}

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("collect one user: %w", err)
	}

	return &user, nil
}

func (us *UserModel) CreateOAuthUser(provider, oauthID, email, oauthEmail string) (*User, error) {
	email = normalizeEmail(email)

	slog.Info("creating oauth user", "provider", provider, "oauth_id", oauthID)

	row := us.Pool.QueryRow(context.Background(), `
		INSERT INTO users (email, oauth_provider, oauth_id, oauth_email) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, subscription_status, stripe_invoice_id
	`, email, provider, oauthID, oauthEmail)

	user := User{
		Email:         email,
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
		OAuthEmail:    &oauthEmail,
	}

	err := row.Scan(&user.ID, &user.SubscriptionStatus, &user.StripeInvoiceId)
	if err != nil {
		var pgErr interface {
			SQLState() string
		}
		if errors.As(err, &pgErr) {
			if pgErr.SQLState() == pgerrcode.UniqueViolation {
				return nil, fmt.Errorf("create oauth user unique violation: %w", err)
			}
		}
		return nil, fmt.Errorf("create oauth user: %w", err)
	}

	return &user, nil
}

func (us *UserModel) LinkOAuthToExistingUser(userID types.UserId, provider, oauthID, oauthEmail string) error {
	_, err := us.Pool.Exec(context.Background(), `
		UPDATE users SET oauth_provider = $1, oauth_id = $2, oauth_email = $3 
		WHERE id = $4
	`, provider, oauthID, oauthEmail, userID)
	if err != nil {
		return fmt.Errorf("link oauth to user: %w", err)
	}
	return nil
}

func (us *UserModel) GetByEmail(email string) (*User, error) {
	email = normalizeEmail(email)
	rows, err := us.Pool.Query(context.Background(), `
		SELECT * FROM users WHERE email = $1
	`, email)

	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found by email: %w", errors.ErrNotFound)
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}
