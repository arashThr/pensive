package auth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/models"
)

type Extension struct {
	TokenModel *models.TokenRepo
}

type ExtensionAuthResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

func (e *Extension) GenerateToken(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	user := usercontext.User(r.Context())
	if user == nil {
		logger.Warnw("extension token generation: unauthenticated request")
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

	logger.Debugw("generating extension token", "user_id", user.ID)
	token, err := e.TokenModel.Create(user.ID, "extension")
	if err != nil {
		logger.Errorw("failed to create extension token", "error", err, "user_id", user.ID)
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

	logger.Infow("extension token generated", "user_id", user.ID)
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
