package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/goauth/cookie"
	"github.com/arashthr/goauth/errors"
	"github.com/arashthr/goauth/models"
)

// PasswordlessConfig configures magic-link (passwordless) authentication handlers.
type PasswordlessConfig struct {
	Domain           string
	TurnstileSecret  string
	TurnstileSiteKey string
	SuccessRedirect  string // Default: "/"

	RenderSignUpForm     RenderFunc
	RenderSignInForm     RenderFunc
	RenderCheckEmail     RenderFunc
}

// PasswordlessHandlers handles email-based magic-link authentication.
type PasswordlessHandlers struct {
	Users      UserStore
	Sessions   SessionStore
	AuthTokens AuthTokenStore
	Email      EmailSender
	Config     PasswordlessConfig
}

// SignUpForm renders the passwordless sign-up form (GET /auth/passwordless/signup).
func (h *PasswordlessHandlers) SignUpForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, h.successRedirect(), http.StatusFound)
		return
	}
	render(h.Config.RenderSignUpForm, w, r, RenderData{
		Title: "Sign Up",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	})
}

// SignUp processes the passwordless sign-up form (POST /auth/passwordless/signup).
func (h *PasswordlessHandlers) SignUp(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	email := r.FormValue("email")
	l.Debugw("passwordless sign-up attempt", "email", email)

	rd := RenderData{
		Title: "Sign Up",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	}

	if err := ValidateTurnstile(r.FormValue("cf-turnstile-response"),
		h.Config.TurnstileSecret, r.RemoteAddr); err != nil {
		l.Warnw("passwordless sign-up turnstile failed", "error", err)
		rd.Flash = &Flash{Message: "Verification failed. Please try again.", IsError: true}
		render(h.Config.RenderSignUpForm, w, r, rd)
		return
	}

	// If the user already exists, send a sign-in link instead.
	if existing, err := h.Users.GetByEmail(email); err == nil && existing != nil {
		rd.Flash = &Flash{Message: "Account already exists. Check your email for a sign-in link.", IsError: false}
		h.sendSignInEmail(email, l)
		render(h.Config.RenderSignUpForm, w, r, rd)
		return
	}

	at, err := h.AuthTokens.Create(email, models.AuthTokenTypeSignup)
	if err != nil {
		l.Errorw("passwordless sign-up – create token failed", "error", err)
		rd.Flash = &Flash{Message: "Failed to send sign-up email. Please try again.", IsError: true}
		render(h.Config.RenderSignUpForm, w, r, rd)
		return
	}

	magicURL := fmt.Sprintf("%s/auth/passwordless/verify?%s",
		h.Config.Domain, url.Values{"token": {at.Token}}.Encode())
	go func() {
		if err := h.Email.PasswordlessSignup(email, magicURL); err != nil {
			l.Errorw("passwordless sign-up – send email failed", "error", err)
		}
	}()

	l.Infow("passwordless sign-up email sent", "email", email)
	render(h.Config.RenderCheckEmail, w, r, RenderData{
		Title: "Check Your Email",
		Data:  map[string]interface{}{"Email": email, "Type": "signup"},
	})
}

// SignInForm renders the passwordless sign-in form (GET /auth/passwordless/signin).
func (h *PasswordlessHandlers) SignInForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, h.successRedirect(), http.StatusFound)
		return
	}
	render(h.Config.RenderSignInForm, w, r, RenderData{
		Title: "Sign In",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	})
}

// SignIn processes the passwordless sign-in form (POST /auth/passwordless/signin).
func (h *PasswordlessHandlers) SignIn(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	email := r.FormValue("email")
	l.Debugw("passwordless sign-in attempt", "email", email)

	rd := RenderData{
		Title: "Sign In",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	}

	if err := ValidateTurnstile(r.FormValue("cf-turnstile-response"),
		h.Config.TurnstileSecret, r.RemoteAddr); err != nil {
		l.Warnw("passwordless sign-in turnstile failed", "error", err)
		rd.Flash = &Flash{Message: "Verification failed. Please try again.", IsError: true}
		render(h.Config.RenderSignInForm, w, r, rd)
		return
	}

	existing, err := h.Users.GetByEmail(email)
	if err != nil || existing == nil {
		rd.Flash = &Flash{Message: "Account not found. Please sign up first.", IsError: true}
		render(h.Config.RenderSignInForm, w, r, rd)
		return
	}

	go func() { h.sendSignInEmail(email, l) }()

	l.Infow("passwordless sign-in email sent", "email", email)
	render(h.Config.RenderCheckEmail, w, r, RenderData{
		Title: "Check Your Email",
		Data:  map[string]interface{}{"Email": email, "Type": "signin"},
	})
}

// Verify processes the magic link (GET /auth/passwordless/verify).
func (h *PasswordlessHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	token := r.URL.Query().Get("token")
	if token == "" {
		l.Warnw("passwordless verify – missing token")
		http.Error(w, "Invalid verification link.", http.StatusBadRequest)
		return
	}

	at, err := h.AuthTokens.Consume(token)
	if err != nil {
		l.Warnw("passwordless verify – consume failed", "error", err)
		http.Error(w, "Invalid or expired verification link.", http.StatusBadRequest)
		return
	}

	var user *models.User
	if at.TokenType == models.AuthTokenTypeSignup {
		user, err = h.Users.CreatePasswordlessUser(r.Context(), at.Email)
		if err != nil {
			if errors.Is(err, errors.ErrEmailTaken) {
				// Race condition: signed up on another device; fall through to sign-in.
				user, err = h.Users.GetByEmail(at.Email)
			}
			if err != nil {
				l.Errorw("passwordless verify – create user failed", "error", err)
				http.Error(w, "Failed to create account.", http.StatusInternalServerError)
				return
			}
		}
	} else {
		user, err = h.Users.GetByEmail(at.Email)
		if err != nil {
			l.Errorw("passwordless verify – get user failed", "error", err)
			http.Error(w, "Account not found.", http.StatusNotFound)
			return
		}
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("passwordless verify – session create failed", "error", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("passwordless verify success", "user_id", user.ID, "type", at.TokenType)
	http.Redirect(w, r, h.successRedirect(), http.StatusFound)
}

func (h *PasswordlessHandlers) sendSignInEmail(email string, l interface {
	Errorw(msg string, args ...interface{})
}) {
	at, err := h.AuthTokens.Create(email, models.AuthTokenTypeSignin)
	if err != nil {
		l.Errorw("passwordless sign-in – create token failed", "error", err)
		return
	}
	magicURL := fmt.Sprintf("%s/auth/passwordless/verify?%s",
		h.Config.Domain, url.Values{"token": {at.Token}}.Encode())
	if err = h.Email.PasswordlessSignin(email, magicURL); err != nil {
		l.Errorw("passwordless sign-in – send email failed", "error", err)
	}
}

func (h *PasswordlessHandlers) successRedirect() string {
	return redirectOrDefault(h.Config.SuccessRedirect, "/")
}
