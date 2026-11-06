package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/arashthr/goauth/handlers"
	"github.com/arashthr/goauth/models"
)

func newPasswordlessHandlers(
	users *mockUserStore,
	sessions *mockSessionStore,
	authTokens *mockAuthTokenStore,
	emailSvc *mockEmailSender,
) *handlers.PasswordlessHandlers {
	return &handlers.PasswordlessHandlers{
		Users:      users,
		Sessions:   sessions,
		AuthTokens: authTokens,
		Email:      emailSvc,
		Config: handlers.PasswordlessConfig{
			Domain:          "https://example.com",
			SuccessRedirect: "/home",
		},
	}
}

// ----- SignUpForm -----

func TestPasswordlessSignUpForm_Renders(t *testing.T) {
	h := newPasswordlessHandlers(newMockUserStore(), newMockSessionStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.SignUpForm(w, newRequest(http.MethodGet, "/auth/passwordless/signup", nil))
	if w.Code != http.StatusOK {
		t.Errorf("PasswordlessSignUpForm: got %d, want 200", w.Code)
	}
}

// ----- SignUp -----

func TestPasswordlessSignUp_NewUser_SendsEmail(t *testing.T) {
	users := newMockUserStore()
	authTokens := newMockAuthTokenStore()
	emailSvc := &mockEmailSender{}
	h := newPasswordlessHandlers(users, newMockSessionStore(), authTokens, emailSvc)

	form := url.Values{"email": {"new@example.com"}}
	w := httptest.NewRecorder()
	h.SignUp(w, newRequest(http.MethodPost, "/auth/passwordless/signup", form))

	if w.Code != http.StatusOK {
		t.Errorf("PasswordlessSignUp new user: got %d, want 200 (check-email page)", w.Code)
	}
}

func TestPasswordlessSignUp_ExistingUser_SendsSignInEmail(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "existing@example.com"})

	authTokens := newMockAuthTokenStore()
	emailSvc := &mockEmailSender{}
	h := newPasswordlessHandlers(users, newMockSessionStore(), authTokens, emailSvc)

	form := url.Values{"email": {"existing@example.com"}}
	w := httptest.NewRecorder()
	h.SignUp(w, newRequest(http.MethodPost, "/auth/passwordless/signup", form))

	// Should still show "check email" (sign-in link was sent)
	if w.Code != http.StatusOK {
		t.Errorf("PasswordlessSignUp existing user: got %d, want 200", w.Code)
	}
}

// ----- SignIn -----

func TestPasswordlessSignIn_ExistingUser_SendsEmail(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "user@example.com"})

	authTokens := newMockAuthTokenStore()
	emailSvc := &mockEmailSender{}
	h := newPasswordlessHandlers(users, newMockSessionStore(), authTokens, emailSvc)

	form := url.Values{"email": {"user@example.com"}}
	w := httptest.NewRecorder()
	h.SignIn(w, newRequest(http.MethodPost, "/auth/passwordless/signin", form))

	if w.Code != http.StatusOK {
		t.Errorf("PasswordlessSignIn: got %d, want 200 (check-email page)", w.Code)
	}
}

func TestPasswordlessSignIn_UnknownUser_ShowsError(t *testing.T) {
	h := newPasswordlessHandlers(newMockUserStore(), newMockSessionStore(), newMockAuthTokenStore(), &mockEmailSender{})
	form := url.Values{"email": {"nobody@example.com"}}
	w := httptest.NewRecorder()
	h.SignIn(w, newRequest(http.MethodPost, "/auth/passwordless/signin", form))

	if w.Code == http.StatusFound {
		t.Error("PasswordlessSignIn with unknown email should not redirect")
	}
}

// ----- Verify -----

func TestPasswordlessVerify_SignupToken_CreatesUserAndSession(t *testing.T) {
	users := newMockUserStore()
	sessions := newMockSessionStore()
	authTokens := newMockAuthTokenStore()

	// Pre-create a signup token
	tok, _ := authTokens.Create("fresh@example.com", models.AuthTokenTypeSignup)

	h := newPasswordlessHandlers(users, sessions, authTokens, &mockEmailSender{})
	w := httptest.NewRecorder()
	req := newRequest(http.MethodGet, "/auth/passwordless/verify?token="+tok.Token, nil)
	h.Verify(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Verify signup: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/home" {
		t.Errorf("Verify signup redirect: got %q, want /home", loc)
	}
	if _, exists := users.users["fresh@example.com"]; !exists {
		t.Error("Verify signup: user was not created")
	}
}

func TestPasswordlessVerify_SigninToken_CreatesSession(t *testing.T) {
	users := newMockUserStore()
	users.add(&models.User{ID: 1, Email: "existing@example.com"})

	sessions := newMockSessionStore()
	authTokens := newMockAuthTokenStore()
	tok, _ := authTokens.Create("existing@example.com", models.AuthTokenTypeSignin)

	h := newPasswordlessHandlers(users, sessions, authTokens, &mockEmailSender{})
	w := httptest.NewRecorder()
	h.Verify(w, newRequest(http.MethodGet, "/auth/passwordless/verify?token="+tok.Token, nil))

	if w.Code != http.StatusFound {
		t.Errorf("Verify signin: got %d, want 302", w.Code)
	}
}

func TestPasswordlessVerify_InvalidToken_Returns400(t *testing.T) {
	h := newPasswordlessHandlers(newMockUserStore(), newMockSessionStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.Verify(w, newRequest(http.MethodGet, "/auth/passwordless/verify?token=nonexistent", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Verify invalid token: got %d, want 400", w.Code)
	}
}

func TestPasswordlessVerify_MissingToken_Returns400(t *testing.T) {
	h := newPasswordlessHandlers(newMockUserStore(), newMockSessionStore(), newMockAuthTokenStore(), &mockEmailSender{})
	w := httptest.NewRecorder()
	h.Verify(w, newRequest(http.MethodGet, "/auth/passwordless/verify", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Verify missing token: got %d, want 400", w.Code)
	}
}
