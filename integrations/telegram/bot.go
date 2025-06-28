package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/errors"
	internalModels "github.com/arashthr/go-course/internal/models"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	apiEndpoint     string
	httpClient      = &http.Client{Timeout: 10 * time.Second}
	userAPITokens   = map[int64]string{}
	telegramService *internalModels.TelegramService
	TokenModel      *internalModels.TokenModel
	configs         *config.AppConfig
)

type BookmarkResponse struct {
	Id            string     `json:"id"`
	Title         string     `json:"title"`
	Link          string     `json:"link"`
	Excerpt       string     `json:"excerpt"`
	CreatedAt     time.Time  `json:"createdAt"`
	PublishedTime *time.Time `json:"publishedTime,omitempty"`
}

type SearchResult struct {
	Headline   string  `json:"headline"`
	BookmarkId string  `json:"bookmarkId"`
	Title      string  `json:"title"`
	Link       string  `json:"link"`
	Excerpt    string  `json:"excerpt"`
	Rank       float32 `json:"rank"`
}

type SearchResponse struct {
	Bookmarks []SearchResult `json:"bookmarks"`
}

func StartBot(telegramToken string, endpoint string, pool *pgxpool.Pool) {
	apiEndpoint = endpoint
	b, err := bot.New(telegramToken)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}

	telegramService = &internalModels.TelegramService{
		Pool: pool,
	}
	TokenModel = &internalModels.TokenModel{
		Pool: pool,
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypePrefix, startHandler)

	b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, handleMessage)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, handleCallbackQuery)

	b.Start(context.Background())
}

func startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	fmt.Printf("Received message: %s\n", update.Message.Text)
	chatId := update.Message.From.ID

	if userAPITokens[chatId] != "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Your account is already connected. You can send links to save them to Pensieve.",
		})
		return
	}

	parts := strings.Split(update.Message.Text, " ")
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please use the link from the Pensieve website to connect your account.",
		})
		return
	}
	authToken := parts[1]

	userId, err := telegramService.GetUserFromAuthToken(authToken)
	if err != nil {
		slog.Error("failed to find auth token", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Invalid or expired authentication link. Please try again from the Pensieve website.",
		})
		return
	}

	token, err := TokenModel.Create(userId)
	if err != nil {
		slog.Error("failed to create API token", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to create API token. Please try again.",
		})
		return
	}

	err = telegramService.SetTokenForChatId(userId, update.Message.Chat.ID, token)
	if err != nil {
		slog.Error("failed to store chat ID", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Failed to connect your account. Please try again.",
		})
		return
	}

	userAPITokens[chatId] = token.Token

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Your Telegram account has been connected! You can now send links to save them to Pensieve.",
	})
}

func handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId := update.Message.From.ID
	if !isUserAuthenticated(userId) {
		integrationsPath, err := url.JoinPath(apiEndpoint, "integrations")
		if err != nil {
			slog.Error("failed to create integrations path", "error", err)
			return
		}
		fmt.Println(integrationsPath)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      fmt.Sprintf(`Please visit Telegram authentication page for authentication: %s`, integrationsPath),
			ParseMode: models.ParseModeMarkdown,
		})
		return
	}

	msg := update.Message.Text
	// Chech message length
	if len(msg) > 1000 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Message is too long. Please shorten it to 1000 characters or less.",
		})
		return
	}

	// Look for a link in the message
	var link string
	parts := strings.Split(msg, " ")
	for _, part := range parts {
		if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
			link = part
			break
		}
	}

	// If there is no link, check if there are too many search terms
	if link == "" && len(parts) > 10 {
		// Too many search terms
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Too many search terms. Please provide a shorter query.",
		})
		return
	}

	if link != "" {
		saveBookmark(ctx, b, update.Message.Chat.ID, link)
	} else {
		searchBookmarks(ctx, b, update.Message.Chat.ID, msg)
	}
}

func saveBookmark(ctx context.Context, b *bot.Bot, chatID int64, link string) {
	reqBody, _ := json.Marshal(map[string]string{"link": link})
	req, err := http.NewRequest("POST", apiEndpoint+"/api/v1/bookmarks", bytes.NewBuffer(reqBody))
	if err != nil {
		slog.Error("failed to create request", "error", err, "link", link, "chatID", chatID)
		return
	}
	req.Header.Set("Authorization", "Bearer "+userAPITokens[chatID])
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send request", "error", err, "link", link, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to save bookmark: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to save bookmark", "status", resp.Status, "link", link, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to save bookmark: " + resp.Status})
		return
	}

	var bookmark BookmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil {
		slog.Error("failed to decode response", "error", err, "link", link, "chatID", chatID)
		return
	}
	slog.Info("Saved bookmark", "id", bookmark.Id, "title", bookmark.Title)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Bookmark saved: " + bookmark.Title,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "Delete", CallbackData: "delete|" + bookmark.Id},
					{Text: "Summary", CallbackData: "summary|" + bookmark.Id},
				},
			},
		},
	})
}

func searchBookmarks(ctx context.Context, b *bot.Bot, chatID int64, query string) {
	req, err := http.NewRequest("GET", apiEndpoint+"/api/v1/bookmarks/search?query="+urlQueryEscape(query), nil)
	if err != nil {
		slog.Error("failed to create request", "error", err, "query", query, "chatID", chatID)
		return
	}
	req.Header.Set("Authorization", "Bearer "+userAPITokens[chatID])

	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("failed to send request", "error", err, "query", query, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Search failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to search bookmarks", "status", resp.Status, "query", query, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Search failed: " + resp.Status})
		return
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("failed to decode response", "error", err, "query", query, "chatID", chatID)
		return
	}

	if len(result.Bookmarks) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "No results found."})
		return
	}

	var sb strings.Builder
	for i, r := range result.Bookmarks {
		sb.WriteString(fmt.Sprintf("%d. <a href=\"%s\">%s</a>\n%s\n\n", i+1, r.Link, r.Title, r.Headline))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: sb.String(), ParseMode: models.ParseModeHTML, LinkPreviewOptions: &models.LinkPreviewOptions{
		IsDisabled: bot.True(),
	}})
}

func handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId := update.CallbackQuery.From.ID
	if !isUserAuthenticated(userId) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Please send your API token to connect your account.",
			ShowAlert:       true,
		})
		return
	}

	data := update.CallbackQuery.Data
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		slog.Error("invalid callback data", "data", data)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Invalid callback",
			ShowAlert:       true,
		})
		return
	}
	action, bookmarkID := parts[0], parts[1]

	switch action {
	case "delete":
		deleteBookmark(ctx, b, update, bookmarkID)
	case "summary":
		getSummary(ctx, b, update, bookmarkID)
	}
}

func deleteBookmark(ctx context.Context, b *bot.Bot, update *models.Update, bookmarkID string) {
	userId := update.CallbackQuery.From.ID
	slog.Debug("Deleting bookmark", "id", bookmarkID, "userId", userId)
	req, _ := http.NewRequest("DELETE", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPITokens[userId])

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		slog.Error("failed to delete bookmark", "error", err, "status", resp.StatusCode)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Failed to delete bookmark",
			ShowAlert:       true,
		})
		return
	}
	defer resp.Body.Close()

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      "Bookmark deleted",
	})
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "Bookmark deleted",
		ShowAlert:       true,
	})
}

func getSummary(ctx context.Context, b *bot.Bot, update *models.Update, bookmarkID string) {
	userId := update.CallbackQuery.From.ID
	slog.Debug("Getting bookmark summary", "id", bookmarkID, "userId", userId)
	req, _ := http.NewRequest("GET", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPITokens[userId])

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		slog.Error("failed to get bookmark summary", "error", err, "status", resp.StatusCode, "bookmarkID", bookmarkID)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Failed to get summary",
			ShowAlert:       true,
		})
		return
	}
	defer resp.Body.Close()

	var bookmark BookmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil {
		slog.Error("failed to decode bookmark", "error", err, "bookmarkID", bookmarkID)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Failed to get summary",
			ShowAlert:       true,
		})
		return
	}

	slog.Debug("Got bookmark summary", "id", bookmarkID, "title", bookmark.Title)
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.CallbackQuery.Message.Message.Chat.ID,
		Text:   fmt.Sprintf("%s\n%s\n\n%s", bookmark.Title, bookmark.Link, bookmark.Excerpt),
	})
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "+", "%2B")
}

func isUserAuthenticated(userId int64) bool {
	if userAPITokens[userId] != "" {
		return true
	}
	userAPITokens[userId] = telegramService.GetToken(userId)
	return userAPITokens[userId] != ""
}
