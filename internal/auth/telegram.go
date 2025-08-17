package auth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
)

type Telegram struct {
	TelegramModel *models.TelegramService
	BotName       string
}

func (t *Telegram) RedirectWithAuthToken(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())

	token, err := t.TelegramModel.CreateAuthToken(user.ID)
	if err != nil {
		logging.Logger.Errorw("failed to create auth token", "error", err, "user", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Redirect to Telegram with the auth token
	telegramURL := fmt.Sprintf("https://t.me/%s?start=%s", t.BotName, token)
	http.Redirect(w, r, telegramURL, http.StatusSeeOther)
}
