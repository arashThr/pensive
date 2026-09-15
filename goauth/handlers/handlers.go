// Package handlers provides HTTP handlers for all authentication flows.
//
// Each handler group accepts interfaces for its dependencies (UserStore,
// SessionStore, etc.) so that callers can inject real implementations (the
// models.*Repo types) or mock implementations in tests.
//
// Rendering is delegated to a RenderFunc provided by the caller so that the
// module stays template-engine-agnostic. A minimal JSON fallback is used when
// RenderFunc is nil.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/arashthr/goauth/context/loggercontext"
	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/models"
	"go.uber.org/zap"
)

// ----- Rendering -----

// RenderData is the shape passed to every RenderFunc.
type RenderData struct {
	Title string
	Flash *Flash
	Data  interface{}
}

// Flash holds a single user-facing notification.
type Flash struct {
	Message string
	IsError bool
}

// RenderFunc renders a page to the HTTP response.
type RenderFunc func(w http.ResponseWriter, r *http.Request, data RenderData)

// defaultRender falls back to a JSON dump when the caller provides no RenderFunc.
func defaultRender(w http.ResponseWriter, _ *http.Request, data RenderData) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func render(fn RenderFunc, w http.ResponseWriter, r *http.Request, data RenderData) {
	if fn != nil {
		fn(w, r, data)
		return
	}
	defaultRender(w, r, data)
}

// redirectOrDefault returns path if non-empty, otherwise returns fallback.
func redirectOrDefault(path, fallback string) string {
	if path != "" {
		return path
	}
	return fallback
}

// ----- Repository interfaces -----

// UserStore is the subset of *models.UserRepo used by the handlers.
type UserStore interface {
	Create(email, password string) (*models.User, error)
	Get(id models.UserID) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByOAuth(provider, oauthID string) (*models.User, error)
	Authenticate(email, password string) (*models.User, error)
	UpdatePassword(id models.UserID, password string) error
	CreateOAuthUser(provider, oauthID, email, oauthEmail string) (*models.User, error)
	CreatePasswordlessUser(ctx context.Context, email string) (*models.User, error)
	LinkOAuthToExistingUser(id models.UserID, provider, oauthID, oauthEmail string) error
	MarkEmailVerified(id models.UserID) error
	Delete(id models.UserID) error
}

// SessionStore is the subset of *models.SessionRepo used by the handlers.
type SessionStore interface {
	Create(userID models.UserID, ipAddress string) (*models.Session, error)
	User(token string) (*models.User, error)
	Delete(token string) error
}

// PasswordResetStore is the subset of *models.PasswordResetRepo used by the handlers.
type PasswordResetStore interface {
	Create(email string) (*models.PasswordReset, error)
	Consume(token string) (*models.User, error)
}

// AuthTokenStore is the subset of *models.AuthTokenService used by the handlers.
type AuthTokenStore interface {
	Create(email string, tokenType models.AuthTokenType) (*models.AuthToken, error)
	Consume(token string) (*models.AuthToken, error)
}

// EmailSender is the subset of *email.Service used by the handlers.
type EmailSender interface {
	ForgotPassword(to, resetURL string) error
	PasswordlessSignup(to, magicURL string) error
	PasswordlessSignin(to, magicURL string) error
	EmailVerification(to, verificationURL string) error
}

// TokenStore is the subset of *models.TokenRepo used by the handlers.
type TokenStore interface {
	Create(userID models.UserID, source string) (*models.GeneratedApiToken, error)
	Get(userID models.UserID) ([]models.ApiToken, error)
	Delete(userID models.UserID, tokenID string) error
}

// TelegramStore is the subset of *models.TelegramRepo used by the handlers.
type TelegramStore interface {
	CreateAuthToken(userID models.UserID) (string, error)
}

// ----- Shared context helpers -----

func currentUser(r *http.Request) *models.User {
	return usercontext.User(r.Context())
}

func log(r *http.Request) *zap.SugaredLogger {
	return loggercontext.Logger(r.Context())
}
