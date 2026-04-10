package handlers

import (
	"fmt"
	"net/http"
)

// TelegramConfig configures Telegram auth.
type TelegramConfig struct {
	BotName string // e.g. "MyAppBot"
}

// TelegramHandler redirects authenticated users to the Telegram bot with a
// short-lived auth token so the bot can link their Telegram account.
type TelegramHandler struct {
	Telegram TelegramStore
	Config   TelegramConfig
}

// Redirect creates an auth token and redirects to the Telegram deep-link
// (GET /telegram/auth). Requires a logged-in user.
func (h *TelegramHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	user := currentUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := h.Telegram.CreateAuthToken(user.ID)
	if err != nil {
		l.Errorw("telegram auth – create token failed", "error", err, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	l.Infow("telegram auth – redirecting user", "user_id", user.ID)
	http.Redirect(w, r,
		fmt.Sprintf("https://t.me/%s?start=%s", h.Config.BotName, token),
		http.StatusSeeOther)
}
