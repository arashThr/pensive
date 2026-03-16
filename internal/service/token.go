package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/config"
	"github.com/arashthr/pensive/internal/models"
)

type Token struct {
	TokenModel  *models.TokenRepo
	UserModel   *models.UserRepo
	Environment config.AppEnv
}

func (t *Token) AuthenticatedPing(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("authenticated pong"))
}

// IssueTokenWithPassword issues an API token by verifying email + password.
//
// Security note: this endpoint is intentionally disabled in production.
//
// @Summary Issue API token with credentials (dev-only)
// @Description Verifies email/password and returns a new API token. Disabled in production.
// @Accept json
// @Produce json
// @Param body body struct{Email string `json:"email"`; Password string `json:"password"`} true "Credentials"
// @Success 200 {object} struct{Token string `json:"token"`; TokenType string `json:"tokenType"`} "Issued token"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid credentials"
// @Failure 403 {object} ErrorResponse "Disabled"
// @Failure 500 {object} ErrorResponse "Internal error"
// @Router /api/v1/tokens/issue [post]
func (t *Token) IssueTokenWithPassword(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())

	if t.Environment == config.EnvProduction {
		_ = writeErrorResponse(w, http.StatusForbidden, ErrorResponse{
			Code:    "DISABLED",
			Message: "Token issuance by password is disabled in production",
		})
		return
	}
	if t.UserModel == nil {
		logger.Errorw("user model not configured for token issuance")
		_ = writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "MISCONFIGURED",
			Message: "Server misconfiguration",
		})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		_ = writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_JSON",
			Message: "Invalid JSON body",
		})
		return
	}
	if strings.TrimSpace(req.Email) == "" || req.Password == "" {
		_ = writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "email and password are required",
		})
		return
	}

	user, err := t.UserModel.Authenticate(req.Email, req.Password)
	if err != nil {
		logger.Infow("token issuance failed", "email", strings.ToLower(strings.TrimSpace(req.Email)), "error", err)
		_ = writeErrorResponse(w, http.StatusUnauthorized, ErrorResponse{
			Code:    "UNAUTHORIZED",
			Message: "Email address or password is incorrect",
		})
		return
	}

	generated, err := t.TokenModel.Create(user.ID, "password")
	if err != nil {
		logger.Errorw("failed to create api token", "error", err, "user", user.ID)
		_ = writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "Failed to issue token",
		})
		return
	}

	var resp struct {
		Token     string `json:"token"`
		TokenType string `json:"tokenType"`
	}
	resp.Token = generated.Token
	tokenType := "Bearer"
	resp.TokenType = tokenType

	if err := writeResponse(w, resp); err != nil {
		logger.Errorw("write response", "error", err)
	}
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
