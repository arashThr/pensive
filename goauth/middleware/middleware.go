package middleware

import (
	"net/http"
	"strings"

	"github.com/arashthr/goauth/context/loggercontext"
	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/cookie"
	"github.com/arashthr/goauth/models"
	"go.uber.org/zap"
)

// ----- Session middleware -----

// SessionMiddleware resolves a session cookie to a user and injects them into
// the request context. It does not enforce authentication.
type SessionMiddleware struct {
	Sessions *models.SessionRepo
}

// SetUser reads the session cookie, resolves it to a user, and injects the
// user into the request context. Continues even if no session is found.
func (m *SessionMiddleware) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := cookie.Read(r, cookie.SessionCookieName)
		if err != nil || token == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := m.Sessions.User(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		r = r.WithContext(usercontext.WithUser(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}

// RequireUser rejects requests where no user has been injected into the context.
// Redirects to signInPath (default "/signin") if not set.
func (m *SessionMiddleware) RequireUser(signInPath string) func(http.Handler) http.Handler {
	if signInPath == "" {
		signInPath = "/signin"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if usercontext.User(r.Context()) == nil {
				http.Redirect(w, r, signInPath, http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----- API-token middleware -----

// APIMiddleware resolves a Bearer token to a user via the api_tokens table.
type APIMiddleware struct {
	Tokens *models.TokenRepo
}

// SetUser reads the Authorization: Bearer <token> header, resolves it to a
// user, and injects the user into the request context.
func (m *APIMiddleware) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			next.ServeHTTP(w, r)
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := m.Tokens.User(parts[1])
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		r = r.WithContext(usercontext.WithUser(r.Context(), user))
		next.ServeHTTP(w, r)
	})
}

// RequireUser returns 401 if no user was found in the context.
func (m *APIMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if usercontext.User(r.Context()) == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ----- Admin basic-auth middleware -----

// AdminMiddleware enforces HTTP Basic Authentication.
type AdminMiddleware struct {
	Username string
	Password string
}

// Require checks the request's Basic Auth credentials.
func (m *AdminMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			!strings.EqualFold(strings.TrimSpace(user), strings.TrimSpace(m.Username)) ||
			!strings.EqualFold(strings.TrimSpace(pass), strings.TrimSpace(m.Password)) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ----- Logger middleware -----

// InjectLogger injects a request-scoped zap logger into the context.
// The logger includes the user ID if a user is already in the context.
func InjectLogger(base *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := base
			if user := usercontext.User(r.Context()); user != nil {
				logger = base.With("user_id", user.ID)
			}
			r = r.WithContext(loggercontext.WithLogger(r.Context(), logger))
			next.ServeHTTP(w, r)
		})
	}
}
