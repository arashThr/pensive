package handlers

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/goauth/models"
)

// VerificationConfig configures the email verification handlers.
type VerificationConfig struct {
	Domain          string
	SuccessRedirect string // Default: "/"
}

// VerificationHandlers manages email verification flows.
type VerificationHandlers struct {
	Users      UserStore
	AuthTokens AuthTokenStore
	Email      EmailSender
	Config     VerificationConfig
}

// VerifyEmail processes a verification link (GET /auth/verify-email).
func (h *VerificationHandlers) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	token := r.URL.Query().Get("token")
	if token == "" {
		l.Warnw("verify email – missing token")
		http.Error(w, "Invalid verification link.", http.StatusBadRequest)
		return
	}

	at, err := h.AuthTokens.Consume(token)
	if err != nil {
		l.Warnw("verify email – consume failed", "error", err)
		http.Error(w, "Invalid or expired verification link.", http.StatusBadRequest)
		return
	}

	if at.TokenType != models.AuthTokenTypeEmailVerification {
		l.Warnw("verify email – wrong token type", "type", at.TokenType)
		http.Error(w, "Invalid verification link.", http.StatusBadRequest)
		return
	}

	user, err := h.Users.GetByEmail(at.Email)
	if err != nil {
		l.Errorw("verify email – get user failed", "error", err)
		http.Error(w, "User not found.", http.StatusNotFound)
		return
	}

	if user.EmailVerified {
		l.Infow("email already verified", "user_id", user.ID)
		http.Redirect(w, r, h.successRedirect(), http.StatusFound)
		return
	}

	if err = h.Users.MarkEmailVerified(user.ID); err != nil {
		l.Errorw("verify email – mark verified failed", "error", err)
		http.Error(w, "Verification failed.", http.StatusInternalServerError)
		return
	}

	l.Infow("email verified", "user_id", user.ID)
	http.Redirect(w, r, h.successRedirect(), http.StatusFound)
}

// ResendVerification resends the verification email (POST /auth/resend-verification).
func (h *VerificationHandlers) ResendVerification(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	user := currentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if user.EmailVerified {
		http.Error(w, "Email already verified.", http.StatusBadRequest)
		return
	}

	if err := h.sendVerificationEmail(user.Email); err != nil {
		l.Errorw("resend verification – send failed", "error", err)
		http.Error(w, "Failed to send verification email.", http.StatusInternalServerError)
		return
	}

	l.Infow("verification email resent", "user_id", user.ID)
	w.WriteHeader(http.StatusOK)
}

// SendVerificationEmail is a helper for creating and emailing a verification token.
// Exported so the application can call it after sign-up if desired.
func (h *VerificationHandlers) SendVerificationEmail(email string) error {
	return h.sendVerificationEmail(email)
}

func (h *VerificationHandlers) sendVerificationEmail(email string) error {
	at, err := h.AuthTokens.Create(email, models.AuthTokenTypeEmailVerification)
	if err != nil {
		return fmt.Errorf("verification – create token: %w", err)
	}
	verifyURL := fmt.Sprintf("%s/auth/verify-email?%s",
		h.Config.Domain, url.Values{"token": {at.Token}}.Encode())
	return h.Email.EmailVerification(email, verifyURL)
}

func (h *VerificationHandlers) successRedirect() string {
	return redirectOrDefault(h.Config.SuccessRedirect, "/")
}
