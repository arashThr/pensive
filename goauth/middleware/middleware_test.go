package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/middleware"
	"github.com/arashthr/goauth/models"
)

// ----- AdminMiddleware -----

func TestAdminMiddleware_ValidCredentials(t *testing.T) {
	amw := &middleware.AdminMiddleware{Username: "admin", Password: "secret"}
	handler := amw.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("AdminMiddleware valid: got %d, want 200", w.Code)
	}
}

func TestAdminMiddleware_InvalidCredentials(t *testing.T) {
	amw := &middleware.AdminMiddleware{Username: "admin", Password: "secret"}
	handler := amw.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AdminMiddleware invalid: got %d, want 401", w.Code)
	}
}

func TestAdminMiddleware_MissingCredentials(t *testing.T) {
	amw := &middleware.AdminMiddleware{Username: "admin", Password: "secret"}
	handler := amw.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("AdminMiddleware missing: got %d, want 401", w.Code)
	}
}

// ----- SessionMiddleware.RequireUser -----

func TestSessionMiddleware_RequireUser_WithUser(t *testing.T) {
	smw := &middleware.SessionMiddleware{}
	handler := smw.RequireUser("/signin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = req.WithContext(usercontext.WithUser(req.Context(), &models.User{ID: 1}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("RequireUser with user: got %d, want 200", w.Code)
	}
}

func TestSessionMiddleware_RequireUser_NoUser(t *testing.T) {
	smw := &middleware.SessionMiddleware{}
	handler := smw.RequireUser("/signin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("RequireUser no user: got %d, want 302", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/signin" {
		t.Errorf("RequireUser no user redirect: got %q, want /signin", loc)
	}
}

// ----- APIMiddleware.RequireUser -----

func TestAPIMiddleware_RequireUser_WithUser(t *testing.T) {
	amw := &middleware.APIMiddleware{}
	handler := amw.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req = req.WithContext(usercontext.WithUser(req.Context(), &models.User{ID: 1}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("APIMiddleware RequireUser with user: got %d, want 200", w.Code)
	}
}

func TestAPIMiddleware_RequireUser_NoUser(t *testing.T) {
	amw := &middleware.APIMiddleware{}
	handler := amw.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("APIMiddleware RequireUser no user: got %d, want 401", w.Code)
	}
}
