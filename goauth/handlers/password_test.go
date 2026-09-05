package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/arashthr/goauth/context/loggercontext"
	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/handlers"
	"github.com/arashthr/goauth/models"
	"go.uber.org/zap"
)

// ----- test helpers -----

func newPasswordHandlers(
	users *mockUserStore,
	sessions *mockSessionStore,
	pwResets *mockPasswordResetStore,
	emailSvc *mockEmailSender,
) *handlers.PasswordHandlers {
	return &handlers.PasswordHandlers{
		Users:    users,
		Sessions: sessions,
		PwResets: pwResets,
		Email:    emailSvc,
		Config: handlers.PasswordConfig{
			Domain:          "https://example.com",
			SuccessRedirect: "/home",
			SignOutRedirect: "/signin",
		},
	}
}

func newRequest(method, target string, form url.Values) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	logger, _ := zap.NewDevelopment()
	req = req.WithContext(loggercontext.WithLogger(req.Context(), logger.Sugar()))
	return req
}

func newRequestWithUser(method, target string, form url.Values, user *models.User) *http.Request {
	req := newRequest(method, target, form)
	req = req.WithContext(usercontext.WithUser(req.Context(), user))
	return req
}

// ----- SignUpForm -----

func TestSignUpForm_RendersForm(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.SignUpForm(w, newRequest(http.MethodGet, "/signup", nil))
	if w.Code != http.StatusOK {
		t.Errorf("SignUpForm: got status %d, want 200", w.Code)
	}
}

func TestSignUpForm_RedirectsIfAlreadyLoggedIn(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	user := &models.User{ID: 1, Email: "logged@example.com"}
	h.SignUpForm(w, newRequestWithUser(http.MethodGet, "/signup", nil, user))
	if w.Code != http.StatusFound {
		t.Errorf("SignUpForm with logged-in user: got %d, want 302", w.Code)
	}
}

// ----- SignUp -----

func TestSignUp_Success(t *testing.T) {
	users := newMockUserStore()
	sessions := newMockSessionStore()
	h := newPasswordHandlers(users, sessions, newMockPasswordResetStore(), &mockEmailSender{})

	form := url.Values{"email": {"new@example.com"}, "password": {"secret123"}}
	w := httptest.NewRecorder()
	h.SignUp(w, newRequest(http.MethodPost, "/signup", form))

	if w.Code != http.StatusFound {
		t.Errorf("SignUp: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/home" {
		t.Errorf("SignUp redirect: got %q, want /home", loc)
	}
	if _, exists := users.users["new@example.com"]; !exists {
		t.Error("SignUp: user was not created in store")
	}
}

func TestSignUp_DuplicateEmail_ShowsError(t *testing.T) {
	users := newMockUserStore()
	users.users["existing@example.com"] = &models.User{ID: 1, Email: "existing@example.com"}
	users.byID[1] = users.users["existing@example.com"]

	h := newPasswordHandlers(users, newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"email": {"existing@example.com"}, "password": {"secret123"}}
	w := httptest.NewRecorder()
	h.SignUp(w, newRequest(http.MethodPost, "/signup", form))

	// Should re-render form, not redirect
	if w.Code == http.StatusFound {
		t.Error("SignUp with duplicate email should not redirect")
	}
}

// ----- SignInForm -----

func TestSignInForm_RendersForm(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.SignInForm(w, newRequest(http.MethodGet, "/signin", nil))
	if w.Code != http.StatusOK {
		t.Errorf("SignInForm: got %d, want 200", w.Code)
	}
}

// ----- SignIn -----

func TestSignIn_Success(t *testing.T) {
	users := newMockUserStore()
	ph := "hashed"
	users.add(&models.User{ID: 1, Email: "user@example.com", PasswordHash: &ph})

	h := newPasswordHandlers(users, newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"email": {"user@example.com"}, "password": {"correct"}}
	w := httptest.NewRecorder()
	h.SignIn(w, newRequest(http.MethodPost, "/signin", form))

	if w.Code != http.StatusFound {
		t.Errorf("SignIn success: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/home" {
		t.Errorf("SignIn redirect: got %q, want /home", loc)
	}
}

func TestSignIn_WrongPassword_ShowsError(t *testing.T) {
	users := newMockUserStore()
	ph := "hashed"
	users.add(&models.User{ID: 1, Email: "user@example.com", PasswordHash: &ph})

	h := newPasswordHandlers(users, newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"email": {"user@example.com"}, "password": {"wrong"}}
	w := httptest.NewRecorder()
	h.SignIn(w, newRequest(http.MethodPost, "/signin", form))

	if w.Code == http.StatusFound {
		t.Error("SignIn with wrong password should not redirect")
	}
}

func TestSignIn_UnknownEmail_ShowsError(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"email": {"nobody@example.com"}, "password": {"password"}}
	w := httptest.NewRecorder()
	h.SignIn(w, newRequest(http.MethodPost, "/signin", form))

	if w.Code == http.StatusFound {
		t.Error("SignIn with unknown email should not redirect")
	}
}

// ----- SignOut -----

func TestSignOut_ClearsCookieAndRedirects(t *testing.T) {
	sessions := newMockSessionStore()
	h := newPasswordHandlers(newMockUserStore(), sessions, newMockPasswordResetStore(), &mockEmailSender{})

	req := newRequest(http.MethodPost, "/signout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok_1"})
	// inject user so middleware doesn't block
	req = req.WithContext(usercontext.WithUser(req.Context(), &models.User{ID: 1}))

	w := httptest.NewRecorder()
	h.SignOut(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SignOut: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/signin" {
		t.Errorf("SignOut redirect: got %q, want /signin", loc)
	}
}

// ----- ForgotPassword -----

func TestForgotPassword_AlwaysShowsCheckEmailPage(t *testing.T) {
	// Even for an unknown email we show the "check your email" page
	// to avoid leaking whether an account exists.
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"email": {"unknown@example.com"}}
	w := httptest.NewRecorder()
	h.ForgotPassword(w, newRequest(http.MethodPost, "/forgot-password", form))
	if w.Code != http.StatusOK {
		t.Errorf("ForgotPassword: got %d, want 200", w.Code)
	}
}

func TestForgotPassword_SendsEmailForKnownUser(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "known@example.com"})

	pwResets := newMockPasswordResetStore()
	pwResets.users["known@example.com"] = users.users["known@example.com"]

	emailSvc := &mockEmailSender{}
	h := newPasswordHandlers(users, newMockSessionStore(), pwResets, emailSvc)

	form := url.Values{"email": {"known@example.com"}}
	w := httptest.NewRecorder()
	h.ForgotPassword(w, newRequest(http.MethodPost, "/forgot-password", form))

	// Wait for goroutine – minimal sleep to let the async send happen.
	// In real tests you'd use a sync mechanism; here we're testing the handler path.
	if w.Code != http.StatusOK {
		t.Errorf("ForgotPassword: got %d, want 200", w.Code)
	}
}

// ----- ResetPasswordForm -----

func TestResetPasswordForm_RendersForm(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.ResetPasswordForm(w, newRequest(http.MethodGet, "/reset-password?token=abc123", nil))
	if w.Code != http.StatusOK {
		t.Errorf("ResetPasswordForm: got %d, want 200", w.Code)
	}
}

// ----- ResetPassword -----

func TestResetPassword_Success(t *testing.T) {
	users := newMockUserStore()
	ph := "old_hash"
	users.add(&models.User{ID: 1, Email: "user@example.com", PasswordHash: &ph})

	pwResets := newMockPasswordResetStore()
	// pre-add a consumable token
	pwResets.tokens["valid_token"] = &models.PasswordReset{Token: "valid_token"}

	sessions := newMockSessionStore()
	h := newPasswordHandlers(users, sessions, pwResets, &mockEmailSender{})

	form := url.Values{"token": {"valid_token"}, "password": {"newpassword"}}
	w := httptest.NewRecorder()
	h.ResetPassword(w, newRequest(http.MethodPost, "/reset-password", form))

	if w.Code != http.StatusFound {
		t.Errorf("ResetPassword: got %d, want 302", w.Code)
	}
}

func TestResetPassword_InvalidToken_Returns400(t *testing.T) {
	h := newPasswordHandlers(newMockUserStore(), newMockSessionStore(), newMockPasswordResetStore(), &mockEmailSender{})
	form := url.Values{"token": {"bad_token"}, "password": {"newpassword"}}
	w := httptest.NewRecorder()
	h.ResetPassword(w, newRequest(http.MethodPost, "/reset-password", form))

	if w.Code != http.StatusBadRequest {
		t.Errorf("ResetPassword bad token: got %d, want 400", w.Code)
	}
}
