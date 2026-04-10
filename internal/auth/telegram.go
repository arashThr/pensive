package auth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/web"
)

type Telegram struct {
	TelegramModel *models.TelegramRepo
	BotName       string
	Templates     struct {
		Connect web.Template
	}
}

func (t *Telegram) RedirectWithAuthToken(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logging.Logger.Debugw("creating Telegram auth token", "user_id", user.ID)

	token, err := t.TelegramModel.CreateAuthToken(user.ID)
	if err != nil {
		logging.Logger.Errorw("failed to create auth token", "error", err, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	telegramURL := fmt.Sprintf("https://t.me/%s?start=%s", t.BotName, token)
	logging.Logger.Infow("showing Telegram connect page", "user_id", user.ID)

	data := struct {
		Title       string
		TelegramURL string
	}{
		Title:       "Connect Telegram",
		TelegramURL: telegramURL,
	}
	t.Templates.Connect.Execute(w, r, data)
}
