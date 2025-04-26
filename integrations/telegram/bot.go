package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var (
	apiToken     string
	apiEndpoint  string
	httpClient   = &http.Client{Timeout: 10 * time.Second}
	userAPIToken = "" // In-memory for demo purposes
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

func main() {
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	apiEndpoint = os.Getenv("API_ENDPOINT")

	if telegramToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}
	if apiEndpoint == "" {
		log.Fatal("API_ENDPOINT is not set")
	}

	slog.Info("Starting Telegram bot")

	b, err := bot.New(telegramToken)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}

	// TODO: Don't process messages until a ping to server is successful
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, func(ctx context.Context, b *bot.Bot, update *models.Update) {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Please provide your API token from " + apiEndpoint + "/users/me",
		})
	})

	b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, handleMessage)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix, handleCallbackQuery)

	b.Start(context.Background())
}

func handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message.Text

	if userAPIToken == "" {
		userAPIToken = msg
		b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    update.Message.Chat.ID,
			MessageID: update.Message.ID,
		})
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Token set! You can now send links to bookmark or text to search.",
		})
		return
	}

	if strings.HasPrefix(msg, "http://") || strings.HasPrefix(msg, "https://") {
		saveBookmark(ctx, b, update.Message.Chat.ID, msg)
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
	req.Header.Set("Authorization", "Bearer "+userAPIToken)
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
	req.Header.Set("Authorization", "Bearer "+userAPIToken)

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
	slog.Info("Deleting bookmark", "id", bookmarkID)
	req, _ := http.NewRequest("DELETE", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPIToken)

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
	req, _ := http.NewRequest("GET", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPIToken)

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
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil || true {
		slog.Error("failed to decode bookmark", "error", err, "bookmarkID", bookmarkID)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Failed to get summary",
			ShowAlert:       true,
		})
		return
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		Text:      fmt.Sprintf("<b>Title</b>: %s\n<em>Link<em>: %s\n\n%s", bookmark.Title, bookmark.Link, bookmark.Excerpt),
		ParseMode: models.ParseModeHTML,
	})
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "+", "%2B")
}
