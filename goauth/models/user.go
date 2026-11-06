package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/arashthr/goauth/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// UserID is the primary key type for users.
type UserID int

// User represents an authenticated user.
type User struct {
	ID              UserID
	Email           string
	PasswordHash    *string // nullable – OAuth / passwordless users have no password
	OAuthProvider   *string
	OAuthID         *string
	OAuthEmail      *string
	EmailVerified   bool
	EmailVerifiedAt *time.Time
}

// HasPassword reports whether the user has a local password set.
func (u *User) HasPassword() bool { return u.PasswordHash != nil }

// IsOAuth reports whether the user authenticated via an OAuth provider.
func (u *User) IsOAuth() bool { return u.OAuthProvider != nil }

// UserRepo handles all database operations for users.
type UserRepo struct {
	Pool *pgxpool.Pool
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Create inserts a new password-based user.
func (r *UserRepo) Create(email, password string) (*User, error) {
	email = normalizeEmail(email)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("create user – hash password: %w", err)
	}
	ph := string(hash)

	var id UserID
	err = r.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, ph,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.ErrEmailTaken
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &User{ID: id, Email: email, PasswordHash: &ph}, nil
}

// Get retrieves a user by ID.
func (r *UserRepo) Get(id UserID) (*User, error) {
	rows, err := r.Pool.Query(context.Background(),
		`SELECT id, email, password_hash, oauth_provider, oauth_id, oauth_email,
		        email_verified, email_verified_at
		 FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByPos[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

// GetByEmail retrieves a user by email address.
func (r *UserRepo) GetByEmail(email string) (*User, error) {
	email = normalizeEmail(email)
	rows, err := r.Pool.Query(context.Background(),
		`SELECT id, email, password_hash, oauth_provider, oauth_id, oauth_email,
		        email_verified, email_verified_at
		 FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByPos[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetByOAuth retrieves a user by OAuth provider + provider-specific ID.
func (r *UserRepo) GetByOAuth(provider, oauthID string) (*User, error) {
	rows, err := r.Pool.Query(context.Background(),
		`SELECT id, email, password_hash, oauth_provider, oauth_id, oauth_email,
		        email_verified, email_verified_at
		 FROM users WHERE oauth_provider = $1 AND oauth_id = $2`, provider, oauthID)
	if err != nil {
		return nil, fmt.Errorf("get user by oauth: %w", err)
	}
	u, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByPos[User])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, fmt.Errorf("get user by oauth: %w", err)
	}
	return u, nil
}

// Authenticate verifies the email+password combination and returns the user.
func (r *UserRepo) Authenticate(email, password string) (*User, error) {
	email = normalizeEmail(email)
	var u User
	err := r.Pool.QueryRow(context.Background(),
		`SELECT id, email, password_hash FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	if u.PasswordHash == nil {
		return nil, fmt.Errorf("authenticate: user has no password set")
	}
	if err = bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	return &u, nil
}

// UpdatePassword updates a user's bcrypt password hash.
func (r *UserRepo) UpdatePassword(id UserID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("update password – hash: %w", err)
	}
	_, err = r.Pool.Exec(context.Background(),
		`UPDATE users SET password_hash = $1 WHERE id = $2`, string(hash), id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// CreateOAuthUser inserts a new user who authenticated via OAuth.
// The user is automatically marked as email-verified.
func (r *UserRepo) CreateOAuthUser(provider, oauthID, email, oauthEmail string) (*User, error) {
	email = normalizeEmail(email)
	var id UserID
	err := r.Pool.QueryRow(context.Background(),
		`INSERT INTO users (email, oauth_provider, oauth_id, oauth_email, email_verified, email_verified_at)
		 VALUES ($1, $2, $3, $4, true, NOW())
		 RETURNING id`,
		email, provider, oauthID, oauthEmail,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.ErrEmailTaken
		}
		return nil, fmt.Errorf("create oauth user: %w", err)
	}
	return &User{
		ID:            id,
		Email:         email,
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
		OAuthEmail:    &oauthEmail,
		EmailVerified: true,
	}, nil
}

// CreatePasswordlessUser inserts a new user who authenticated via magic link.
// The user is automatically marked as email-verified.
func (r *UserRepo) CreatePasswordlessUser(ctx context.Context, email string) (*User, error) {
	email = normalizeEmail(email)
	var id UserID
	err := r.Pool.QueryRow(ctx,
		`INSERT INTO users (email, email_verified, email_verified_at)
		 VALUES ($1, true, NOW())
		 RETURNING id`,
		email,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, errors.ErrEmailTaken
		}
		return nil, fmt.Errorf("create passwordless user: %w", err)
	}
	return &User{ID: id, Email: email, EmailVerified: true}, nil
}

// LinkOAuthToExistingUser attaches OAuth credentials to an existing account.
func (r *UserRepo) LinkOAuthToExistingUser(id UserID, provider, oauthID, oauthEmail string) error {
	_, err := r.Pool.Exec(context.Background(),
		`UPDATE users SET oauth_provider = $1, oauth_id = $2, oauth_email = $3 WHERE id = $4`,
		provider, oauthID, oauthEmail, id)
	if err != nil {
		return fmt.Errorf("link oauth: %w", err)
	}
	return nil
}

// MarkEmailVerified marks the user's email as verified.
func (r *UserRepo) MarkEmailVerified(id UserID) error {
	_, err := r.Pool.Exec(context.Background(),
		`UPDATE users SET email_verified = true, email_verified_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	return nil
}

// Delete permanently removes a user and all cascading data.
func (r *UserRepo) Delete(id UserID) error {
	_, err := r.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// isUniqueViolation reports whether a pgx error is a unique-constraint violation.
func isUniqueViolation(err error) bool {
	type sqlStater interface{ SQLState() string }
	var pgErr sqlStater
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == pgerrcode.UniqueViolation
	}
	return false
}
