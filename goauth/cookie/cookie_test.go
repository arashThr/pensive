package cookie_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arashthr/goauth/cookie"
)

func TestSet_And_Read(t *testing.T) {
	w := httptest.NewRecorder()
	cookie.Set(w, "session", "tok123")

	// Transfer the Set-Cookie header to a new request.
	req := &http.Request{Header: http.Header{}}
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}

	val, err := cookie.Read(req, "session")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if val != "tok123" {
		t.Errorf("Read: got %q, want %q", val, "tok123")
	}
}

func TestRead_Missing(t *testing.T) {
	req := &http.Request{Header: http.Header{}}
	_, err := cookie.Read(req, "session")
	if err == nil {
		t.Error("expected error for missing cookie, got nil")
	}
}

func TestDelete_SetsMaxAgeNegative(t *testing.T) {
	w := httptest.NewRecorder()
	cookie.Delete(w, "session")
	result := w.Result()
	cookies := result.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("MaxAge: got %d, want -1", cookies[0].MaxAge)
	}
}

func TestNew_IsHttpOnly(t *testing.T) {
	c := cookie.New("foo", "bar")
	if !c.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
}
