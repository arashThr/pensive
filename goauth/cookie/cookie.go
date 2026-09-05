package cookie

import (
	"fmt"
	"net/http"
)

const SessionCookieName = "session"

// New creates an HttpOnly cookie.
func New(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
	}
}

// Set writes a cookie to the response.
func Set(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, New(name, value))
}

// Read returns the named cookie value from the request.
func Read(r *http.Request, name string) (string, error) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", fmt.Errorf("read cookie %q: %w", name, err)
	}
	return c.Value, nil
}

// Delete removes a cookie by setting MaxAge=-1.
func Delete(w http.ResponseWriter, name string) {
	c := New(name, "")
	c.MaxAge = -1
	http.SetCookie(w, c)
}
