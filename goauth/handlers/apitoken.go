package handlers

import (
	"fmt"
	"net/http"
)

// APITokenHandlers manages user API token creation and deletion.
type APITokenHandlers struct {
	Tokens TokenStore
}

// GenerateToken creates a new API token for the authenticated user and returns
// it in a minimal HTML page (GET /extension/auth).
//
// The token is only shown once – the client (browser extension, etc.) must
// capture it from the hidden form field before closing the page.
func (h *APITokenHandlers) GenerateToken(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	user := currentUser(r)
	if user == nil {
		l.Warnw("extension token – unauthenticated")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><p>Not authenticated.</p></body></html>`))
		return
	}

	token, err := h.Tokens.Create(user.ID, "extension")
	if err != nil {
		l.Errorw("extension token – create failed", "error", err, "user_id", user.ID)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><p>Failed to generate token.</p></body></html>`))
		return
	}

	l.Infow("extension token generated", "user_id", user.ID)
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authentication</title></head>
<body>
  <h2>Authentication succeeded</h2>
  <p>Your extension is now authenticated.</p>
  <form id="tokenForm">
    <input type="hidden" id="token" name="token" value="%s">
  </form>
</body>
</html>`, token.Token)))
}

// DeleteToken removes an API token by ID (POST /users/delete-token).
func (h *APITokenHandlers) DeleteToken(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	user := currentUser(r)
	tokenID := r.FormValue("token_id")
	l.Debugw("delete API token", "user_id", user.ID, "token_id", tokenID)

	if err := h.Tokens.Delete(user.ID, tokenID); err != nil {
		l.Errorw("delete token failed", "error", err)
		http.Error(w, "Failed to delete token.", http.StatusInternalServerError)
		return
	}

	l.Infow("API token deleted", "user_id", user.ID, "token_id", tokenID)
	w.WriteHeader(http.StatusOK)
}
