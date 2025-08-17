package service

import (
	"net/http"
	"strings"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/models"
)

type Token struct {
	TokenModel *models.TokenRepo
}

func (t *Token) AuthenticatedPing(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("authenticated pong"))
}

// @Summary Delete current token
// @Description Deletes the current token from the database
// @Accept json
// @Produce json
// @Param Authorization header string true "Authorization header"
// @Success 200 {string} string "Token deleted"
// @Failure 400 {string} string "No authorization header"
// @Failure 400 {string} string "Invalid authorization header format"
// @Failure 500 {string} string "Failed to delete token"
// @Router /api/v1/tokens/current [delete]
func (t *Token) DeleteToken(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		logger.Errorw("no authorization header", "authHeader", authHeader)
		http.Error(w, "No authorization header", http.StatusBadRequest)
		return
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		logger.Errorw("invalid authorization header format", "authHeader", authHeader)
		http.Error(w, "Invalid authorization header format", http.StatusBadRequest)
		return
	}

	token := tokenParts[1]
	err := t.TokenModel.DeleteByToken(token)
	if err != nil {
		logger.Errorw("failed to delete current token", "error", err)
		http.Error(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Token deleted"))
}
