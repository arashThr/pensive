package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/handlers"
	"github.com/arashthr/goauth/models"
)

func newVerificationHandlers(users *mockUserStore, authTokens *mockAuthTokenStore, emailSvc *mockEmailSender) *handlers.VerificationHandlers {
	return &handlers.VerificationHandlers{
		Users:      users,
		AuthTokens: authTokens,
		Email:      emailSvc,
		Config: handlers.VerificationConfig{
			Domain:          "https://example.com",
			SuccessRedirect: "/home",
		},
	}
}

// ----- VerifyEmail -----

func TestVerifyEmail_Success(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "user@example.com", EmailVerified: false})

	authTokens := newMockAuthTokenStore()
	tok, _ := authTokens.Create("user@example.com", models.AuthTokenTypeEmailVerification)

	h := newVerificationHandlers(users, authTokens, &mockEmailSender{})
	w := httptest.NewRecorder()
	h.VerifyEmail(w, newRequest(http.MethodGet, "/auth/verify-email?token="+tok.Token, nil))

	if w.Code != http.StatusFound {
		t.Errorf("VerifyEmail: got %d, want 302", w.Code)
	}
	if !users.users["user@example.com"].EmailVerified {
		t.Error("VerifyEmail: email was not marked as verified")
	}
}

func TestVerifyEmail_AlreadyVerified_Redirects(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "user@example.com", EmailVerified: true})

	authTokens := newMockAuthTokenStore()
	tok, _ := authTokens.Create("user@example.com", models.AuthTokenTypeEmailVerification)

	h := newVerificationHandlers(users, authTokens, &mockEmailSender{})
	w := httptest.NewRecorder()
	h.VerifyEmail(w, newRequest(http.MethodGet, "/auth/verify-email?token="+tok.Token, nil))

	// Should still redirect to home without error
	if w.Code != http.StatusFound {
		t.Errorf("VerifyEmail already verified: got %d, want 302", w.Code)
	}
}

func TestVerifyEmail_InvalidToken_Returns400(t *testing.T) {
	h := newVerificationHandlers(newMockUserStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.VerifyEmail(w, newRequest(http.MethodGet, "/auth/verify-email?token=badtoken", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("VerifyEmail invalid token: got %d, want 400", w.Code)
	}
}

func TestVerifyEmail_MissingToken_Returns400(t *testing.T) {
	h := newVerificationHandlers(newMockUserStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.VerifyEmail(w, newRequest(http.MethodGet, "/auth/verify-email", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("VerifyEmail missing token: got %d, want 400", w.Code)
	}
}

func TestVerifyEmail_WrongTokenType_Returns400(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "user@example.com"})

	authTokens := newMockAuthTokenStore()
	// Use signin type instead of email_verification
	tok, _ := authTokens.Create("user@example.com", models.AuthTokenTypeSignin)

	h := newVerificationHandlers(users, authTokens, &mockEmailSender{})
	w := httptest.NewRecorder()
	h.VerifyEmail(w, newRequest(http.MethodGet, "/auth/verify-email?token="+tok.Token, nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("VerifyEmail wrong token type: got %d, want 400", w.Code)
	}
}

// ----- ResendVerification -----

func TestResendVerification_Success(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "user@example.com", EmailVerified: false})

	authTokens := newMockAuthTokenStore()
	emailSvc := &mockEmailSender{}
	h := newVerificationHandlers(users, authTokens, emailSvc)

	req := newRequest(http.MethodPost, "/auth/resend-verification", nil)
	req = req.WithContext(usercontext.WithUser(req.Context(), users.users["user@example.com"]))

	w := httptest.NewRecorder()
	h.ResendVerification(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ResendVerification: got %d, want 200", w.Code)
	}
}

func TestResendVerification_AlreadyVerified_Returns400(t *testing.T) {
	users := newMockUserStore()
	user := &models.User{ID: 1, Email: "user@example.com", EmailVerified: true}
	users.add(user)

	h := newVerificationHandlers(users, newMockAuthTokenStore(), &mockEmailSender{})

	req := newRequest(http.MethodPost, "/auth/resend-verification", nil)
	req = req.WithContext(usercontext.WithUser(req.Context(), user))

	w := httptest.NewRecorder()
	h.ResendVerification(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ResendVerification already verified: got %d, want 400", w.Code)
	}
}

func TestResendVerification_Unauthenticated_Returns401(t *testing.T) {
	h := newVerificationHandlers(newMockUserStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.ResendVerification(w, newRequest(http.MethodPost, "/auth/resend-verification", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ResendVerification unauthenticated: got %d, want 401", w.Code)
	}
}
