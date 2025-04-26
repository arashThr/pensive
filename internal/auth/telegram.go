package auth

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/models"
)

type Telegram struct {
	TelegramModel *models.TelegramService
	BotName       string
}

func (t *Telegram) RedirectWithAuthToken(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())

	token, err := t.TelegramModel.CreateAuthToken(user.ID)
	if err != nil {
		slog.Error("failed to create auth token", "error", err, "user", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Redirect to Telegram with the auth token
	telegramURL := fmt.Sprintf("https://t.me/%s?start=%s", t.BotName, token)
	http.Redirect(w, r, telegramURL, http.StatusSeeOther)
}
