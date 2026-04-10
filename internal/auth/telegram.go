package auth

import (
	"fmt"
	"net/http"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/web"
)

type Telegram struct {
	TelegramModel *models.TelegramRepo
	TokenModel    *models.TokenRepo
	BotName       string
	Templates     struct {
		Connect        web.Template // web→Telegram flow
		BotConnect     web.Template // Telegram→web flow
	}
}

// RedirectWithAuthToken handles the web-initiated flow: creates a token for the
// logged-in user and shows a page with a button to open the Telegram bot.
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

	data := struct {
		Title       string
		TelegramURL string
	}{
		Title:       "Connect Telegram",
		TelegramURL: telegramURL,
	}
	t.Templates.Connect.Execute(w, r, data)
}

// ConnectPage handles GET /telegram/connect — the bot-initiated flow.
// The Telegram bot sends the user this URL with a short-lived token.
// The user must be logged in (RequireUser middleware handles the redirect).
func (t *Telegram) ConnectPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/integrations", http.StatusSeeOther)
		return
	}
	// Validate the token exists (but don't consume it yet)
	_, err := t.TelegramModel.GetChatIdFromConnectToken(token)
	if err != nil {
		loggercontext.Logger(r.Context()).Warnw("invalid or expired connect token", "error", err)
		data := struct {
			Title     string
			Expired   bool
			Connected bool
			BotName   string
		}{Title: "Connect Telegram", Expired: true, BotName: t.BotName}
		t.Templates.BotConnect.Execute(w, r, data)
		return
	}

	data := struct {
		Title     string
		Token     string
		Expired   bool
		Connected bool
		BotName   string
	}{
		Title:   "Connect Telegram",
		Token:   token,
		BotName: t.BotName,
	}
	t.Templates.BotConnect.Execute(w, r, data)
}

// ProcessConnect handles POST /telegram/connect — consumes the token, links the
// Telegram chat to the logged-in user, and re-renders with a success state.
func (t *Telegram) ProcessConnect(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	user := usercontext.User(r.Context())
	token := r.FormValue("token")

	if token == "" {
		http.Redirect(w, r, "/integrations", http.StatusSeeOther)
		return
	}

	chatId, err := t.TelegramModel.GetChatIdFromConnectToken(token)
	if err != nil {
		logger.Warnw("connect token invalid or expired on POST", "error", err)
		data := struct {
			Title     string
			Expired   bool
			Connected bool
			BotName   string
		}{Title: "Connect Telegram", Expired: true, BotName: t.BotName}
		t.Templates.BotConnect.Execute(w, r, data)
		return
	}

	// Create an API token for this user (used by the Telegram bot)
	apiToken, err := t.TokenModel.Create(user.ID, "telegram")
	if err != nil {
		logger.Errorw("failed to create API token for telegram", "error", err, "user_id", user.ID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Persist the connection
	if err := t.TelegramModel.UpsertConnection(user.ID, chatId, apiToken.Token); err != nil {
		logger.Errorw("failed to upsert telegram connection", "error", err, "user_id", user.ID, "chat_id", chatId)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Clean up the used token
	_ = t.TelegramModel.DeleteConnectToken(token)

	logger.Infow("telegram account connected via bot flow", "user_id", user.ID, "chat_id", chatId)

	data := struct {
		Title     string
		Expired   bool
		Connected bool
		BotName   string
	}{
		Title:     "Telegram connected",
		Connected: true,
		BotName:   t.BotName,
	}
	t.Templates.BotConnect.Execute(w, r, data)
}
