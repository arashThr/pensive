// Package oauth provides OAuth2 handlers for GitHub and Google authentication.
package oauth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/goauth/context/loggercontext"
	"github.com/arashthr/goauth/models"
	"github.com/arashthr/goauth/rand"
	"go.uber.org/zap"
)

// UserStore is the subset of the user repository needed by OAuth handlers.
type UserStore interface {
	GetByOAuth(provider, oauthID string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	CreateOAuthUser(provider, oauthID, email, oauthEmail string) (*models.User, error)
	LinkOAuthToExistingUser(id models.UserID, provider, oauthID, oauthEmail string) error
}

// SessionStore is the subset of the session repository needed by OAuth handlers.
type SessionStore interface {
	Create(userID models.UserID, ipAddress string) (*models.Session, error)
}

// ----- helpers -----

func log(r *http.Request) *zap.SugaredLogger {
	return loggercontext.Logger(r.Context())
}

func redirectOrDefault(path, fallback string) string {
	if path != "" {
		return path
	}
	return fallback
}

// generateState creates a random state string for OAuth CSRF protection.
func generateState() (string, error) {
	return rand.String(32)
}

// oauthStateCookie returns a short-lived cookie used to store the OAuth state
// parameter and prevent CSRF attacks.
func oauthStateCookie(state string) *http.Cookie {
	return &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// clearOAuthStateCookie clears the OAuth state cookie.
func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// verifyOAuthState checks that the state parameter in the callback matches
// the stored cookie, then clears the cookie.
func verifyOAuthState(w http.ResponseWriter, r *http.Request) error {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		return fmt.Errorf("missing oauth_state cookie")
	}
	clearOAuthStateCookie(w)
	if r.URL.Query().Get("state") != stateCookie.Value {
		return fmt.Errorf("oauth state mismatch")
	}
	return nil
}
