package handlers

import (
	"fmt"
	"net/http"
)

// oauthStateCookie returns a short-lived cookie used to store the OAuth state
// parameter and prevent CSRF attacks.
func oauthStateCookie(state string) *http.Cookie {
	return &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// clearOAuthStateCookie clears the OAuth state cookie.
func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// verifyOAuthState checks that the state parameter in the callback matches
// the stored cookie, then clears the cookie.
func verifyOAuthState(w http.ResponseWriter, r *http.Request) error {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		return fmt.Errorf("missing oauth_state cookie")
	}
	clearOAuthStateCookie(w)
	if r.URL.Query().Get("state") != stateCookie.Value {
		return fmt.Errorf("oauth state mismatch")
	}
	return nil
}
