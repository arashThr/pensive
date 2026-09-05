package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/goauth/cookie"
	"github.com/arashthr/goauth/errors"
)

// PasswordConfig configures the password-based auth handlers.
type PasswordConfig struct {
	// Domain is the base URL used to build email links (e.g. "https://example.com").
	Domain string

	// Turnstile – leave TurnstileSecret empty to disable CAPTCHA validation.
	TurnstileSecret  string
	TurnstileSiteKey string // forwarded to render data for the client-side widget

	// Redirects
	SuccessRedirect string // after sign-up / sign-in. Default: "/"
	SignOutRedirect string // after sign-out.        Default: "/signin"

	// RenderXxx – if nil, a minimal JSON response is returned.
	RenderSignUp     RenderFunc
	RenderSignIn     RenderFunc
	RenderForgotPw   RenderFunc
	RenderCheckEmail RenderFunc
	RenderResetPw    RenderFunc
}

// PasswordHandlers handles traditional password-based authentication.
type PasswordHandlers struct {
	Users    UserStore
	Sessions SessionStore
	PwResets PasswordResetStore
	Email    EmailSender
	Config   PasswordConfig
}

// signupData is the render payload for the sign-up page.
type signupData struct{ TurnstileSiteKey string }

// SignUpForm renders the sign-up form (GET /signup).
func (h *PasswordHandlers) SignUpForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, h.successRedirect(), http.StatusFound)
		return
	}
	render(h.Config.RenderSignUp, w, r, RenderData{
		Title: "Sign Up",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	})
}

// SignUp processes the sign-up form (POST /signup).
func (h *PasswordHandlers) SignUp(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	email := r.FormValue("email")
	password := r.FormValue("password")
	l.Debugw("sign-up attempt", "email", email)

	rd := RenderData{
		Title: "Sign Up",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	}

	if err := ValidateTurnstile(r.FormValue("cf-turnstile-response"),
		h.Config.TurnstileSecret, r.RemoteAddr); err != nil {
		l.Warnw("sign-up turnstile failed", "error", err)
		rd.Flash = &Flash{Message: "Verification failed. Please try again.", IsError: true}
		render(h.Config.RenderSignUp, w, r, rd)
		return
	}

	user, err := h.Users.Create(email, password)
	if err != nil {
		if errors.Is(err, errors.ErrEmailTaken) {
			rd.Flash = &Flash{Message: "That email address is already taken.", IsError: true}
		} else {
			l.Errorw("sign-up user create failed", "error", err)
			rd.Flash = &Flash{Message: "Sign-up failed. Please try again.", IsError: true}
		}
		render(h.Config.RenderSignUp, w, r, rd)
		return
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("sign-up session create failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("sign-up success", "user_id", user.ID)
	http.Redirect(w, r, h.successRedirect(), http.StatusFound)
}

// SignInForm renders the sign-in form (GET /signin).
func (h *PasswordHandlers) SignInForm(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, h.successRedirect(), http.StatusFound)
		return
	}
	render(h.Config.RenderSignIn, w, r, RenderData{
		Title: "Sign In",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	})
}

// SignIn processes the sign-in form (POST /signin).
func (h *PasswordHandlers) SignIn(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	email := r.FormValue("email")
	password := r.FormValue("password")
	l.Debugw("sign-in attempt", "email", email)

	rd := RenderData{
		Title: "Sign In",
		Data:  signupData{TurnstileSiteKey: h.Config.TurnstileSiteKey},
	}

	if err := ValidateTurnstile(r.FormValue("cf-turnstile-response"),
		h.Config.TurnstileSecret, r.RemoteAddr); err != nil {
		l.Warnw("sign-in turnstile failed", "error", err)
		rd.Flash = &Flash{Message: "Verification failed. Please try again.", IsError: true}
		render(h.Config.RenderSignIn, w, r, rd)
		return
	}

	user, err := h.Users.Authenticate(email, password)
	if err != nil {
		l.Warnw("sign-in failed", "email", email)
		rd.Flash = &Flash{Message: "Email or password is incorrect.", IsError: true}
		render(h.Config.RenderSignIn, w, r, rd)
		return
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("sign-in session create failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("sign-in success", "user_id", user.ID)
	http.Redirect(w, r, h.successRedirect(), http.StatusFound)
}

// SignOut destroys the session (POST /signout).
func (h *PasswordHandlers) SignOut(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	token, err := cookie.Read(r, cookie.SessionCookieName)
	if err != nil {
		http.Redirect(w, r, h.signOutRedirect(), http.StatusFound)
		return
	}
	if err = h.Sessions.Delete(token); err != nil {
		l.Errorw("sign-out session delete failed", "error", err)
	}
	cookie.Delete(w, cookie.SessionCookieName)
	l.Infow("sign-out success")
	http.Redirect(w, r, h.signOutRedirect(), http.StatusFound)
}

// ForgotPasswordForm renders the forgot-password form (GET /forgot-password).
func (h *PasswordHandlers) ForgotPasswordForm(w http.ResponseWriter, r *http.Request) {
	render(h.Config.RenderForgotPw, w, r, RenderData{
		Title: "Forgot Password",
		Data:  map[string]string{"Email": r.FormValue("email")},
	})
}

// ForgotPassword processes the forgot-password form (POST /forgot-password).
func (h *PasswordHandlers) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	email := r.FormValue("email")
	l.Debugw("forgot password request", "email", email)

	pr, err := h.PwResets.Create(email)
	if err != nil {
		l.Errorw("forgot password – create token failed", "error", err)
		// Don't reveal whether the email exists.
	} else {
		resetURL := fmt.Sprintf("%s/reset-password?%s",
			h.Config.Domain, url.Values{"token": {pr.Token}}.Encode())
		go func() {
			if err := h.Email.ForgotPassword(email, resetURL); err != nil {
				l.Errorw("forgot password – send email failed", "error", err)
			}
		}()
	}

	render(h.Config.RenderCheckEmail, w, r, RenderData{
		Title: "Check Your Email",
		Data:  map[string]string{"Email": email},
	})
}

// ResetPasswordForm renders the reset-password form (GET /reset-password).
func (h *PasswordHandlers) ResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	render(h.Config.RenderResetPw, w, r, RenderData{
		Title: "Reset Password",
		Data:  map[string]string{"Token": r.URL.Query().Get("token")},
	})
}

// ResetPassword processes the new-password form (POST /reset-password).
func (h *PasswordHandlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	token := r.FormValue("token")
	password := r.FormValue("password")

	user, err := h.PwResets.Consume(token)
	if err != nil {
		l.Warnw("reset password – consume token failed", "error", err)
		http.Error(w, "Invalid or expired reset link.", http.StatusBadRequest)
		return
	}

	if err = h.Users.UpdatePassword(user.ID, password); err != nil {
		l.Errorw("reset password – update failed", "error", err)
		http.Error(w, "Password reset failed.", http.StatusInternalServerError)
		return
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("reset password – session create failed", "error", err)
		http.Redirect(w, r, h.signOutRedirect(), http.StatusFound)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("password reset success", "user_id", user.ID)
	http.Redirect(w, r, h.successRedirect(), http.StatusFound)
}

func (h *PasswordHandlers) successRedirect() string {
	return redirectOrDefault(h.Config.SuccessRedirect, "/")
}

func (h *PasswordHandlers) signOutRedirect() string {
	return redirectOrDefault(h.Config.SignOutRedirect, "/signin")
}
