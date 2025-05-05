package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
)

type Extension struct {
	ApiService *models.ApiService
}

type ExtensionAuthResponse struct {
	Token string `json:"token"`
	Error string `json:"error,omitempty"`
}

func (e *Extension) GenerateToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := context.User(r.Context())
	if user == nil {
		json.NewEncoder(w).Encode(ExtensionAuthResponse{
			Error: "User not authenticated",
		})
		return
	}

	token, err := e.ApiService.Create(user.ID)
	if err != nil {
		slog.Error("failed to create extension token", "error", err, "user", user.ID)
		json.NewEncoder(w).Encode(ExtensionAuthResponse{
			Error: "Failed to generate token",
		})
		return
	}

	json.NewEncoder(w).Encode(ExtensionAuthResponse{
		Token: token.Token,
	})
}
