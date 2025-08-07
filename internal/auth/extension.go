package auth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
)

type Extension struct {
	TokenModel *models.TokenModel
}

type ExtensionAuthResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

func (e *Extension) GenerateToken(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		html := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Authentication Error</title>
		</head>
		<body>
			<h2>Error</h2>
			<p>User not authenticated</p>
		</body>
		</html>
		`
		w.Write([]byte(html))
		return
	}

	token, err := e.TokenModel.Create(user.ID, "extension")
	if err != nil {
		logger.Errorw("failed to create extension token", "error", err, "user", user.ID)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		html := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Token Generation Error</title>
		</head>
		<body>
			<h2>Error</h2>
			<p>Failed to generate token</p>
		</body>
		</html>
		`
		w.Write([]byte(html))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<title>Authentication</title>
	</head>
	<body>
		<h2>Authentication succeeded</h2>
		<p>Your extension is now authenticated.</p>
		<form id="tokenForm">
			<input type="hidden" id="token" name="token" value="%s">
		</form>
	</body>
	</html>
	`, token.Token)

	w.Write([]byte(html))
}
