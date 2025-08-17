package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arashthr/pensive/internal/logging"
	internalModels "github.com/arashthr/pensive/internal/models"
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
	Headline string  `json:"headline"`
	Id       string  `json:"id"`
	Title    string  `json:"title"`
	Link     string  `json:"link"`
	Excerpt  string  `json:"excerpt"`
	Rank     float32 `json:"rank"`
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
			Text:   "✅ Your account is already connected!\n\nYou can now:\n• Send links to save them instantly\n• Search your bookmarks by typing keywords\n• Get AI summaries of saved content",
		})
		return
	}

	parts := strings.Split(update.Message.Text, " ")
	if len(parts) != 2 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "🔗 <b>Connect your Pensive account</b>\n\nPlease use the authentication link from your Pensive integrations page to connect your account.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	authToken := parts[1]

	userId, err := telegramService.GetUserFromAuthToken(authToken)
	if err != nil {
		logging.Logger.Errorw("failed to find auth token", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "❌ <b>Authentication failed</b>\n\nYour link is invalid or expired. Please generate a new link from the Pensive integrations page.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	token, err := TokenModel.Create(userId, "telegram")
	if err != nil {
		logging.Logger.Errorw("failed to create API token", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "⚠️ <b>Setup error</b>\n\nFailed to create API token. Please try connecting again.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	err = telegramService.SetTokenForChatId(userId, update.Message.Chat.ID, token)
	if err != nil {
		logging.Logger.Errorw("failed to store chat ID", "error", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "⚠️ <b>Connection error</b>\n\nFailed to complete the connection. Please try again.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	userAPITokens[chatId] = token.Token

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      "🎉 <b>Successfully connected!</b>\n\nYour Pensive account is now linked to Telegram.\n\n<b>What you can do:</b>\n• Send any link to save it to your library\n• Type keywords to search your bookmarks\n• Get AI summaries and manage your content\n\nStart by sending a link or searching for something!",
		ParseMode: models.ParseModeHTML,
	})
}

func handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId := update.Message.From.ID
	if !isUserAuthenticated(userId) {
		integrationsPath, err := url.JoinPath(apiEndpoint, "integrations")
		if err != nil {
			logging.Logger.Errorw("failed to create integrations path", "error", err)
			return
		}
		fmt.Println(integrationsPath)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      fmt.Sprintf("🔐 <b>Account not connected</b>\n\nPlease connect your Pensive account first:\n%s\n\nClick the link above, then follow the connection instructions.", integrationsPath),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	msg := update.Message.Text
	// Chech message length
	if len(msg) > 1000 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      "📝 <b>Message too long</b>\n\nPlease keep your message under 1000 characters for better processing.",
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	// Look for a link in the message
	var link string
	parts := strings.Fields(msg)
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
			ChatID:    update.Message.Chat.ID,
			Text:      "🔍 <b>Search query too long</b>\n\nPlease use fewer keywords for better search results.",
			ParseMode: models.ParseModeHTML,
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
		logging.Logger.Errorw("failed to create save request", "error", err, "link", link, "chatID", chatID)
		return
	}
	req.Header.Set("Authorization", "Bearer "+userAPITokens[chatID])
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		logging.Logger.Errorw("failed to send request", "error", err, "link", link, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ <b>Save failed</b>\n\nNetwork error: " + err.Error(),
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Logger.Errorw("failed to save bookmark", "status", resp.Status, "link", link, "chatID", chatID)

		var errorMessage string
		if resp.StatusCode == http.StatusTooManyRequests {
			errorMessage = "❌ <b>Daily limit exceeded</b>\n\nYou've reached your daily bookmark limit. Upgrade to premium for 100 bookmarks/day."
		} else {
			errorMessage = "❌ <b>Save failed</b>\n\nServer error: " + resp.Status
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      errorMessage,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var bookmark BookmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil {
		logging.Logger.Errorw("failed to decode response", "error", err, "link", link, "chatID", chatID)
		return
	}
	logging.Logger.Infow("Saved bookmark", "id", bookmark.Id, "title", bookmark.Title)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("✅ <b>Saved successfully!</b>\n\n<a href=\"%s\">%s</a>", bookmark.Link, bookmark.Title),
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "🗑 Delete", CallbackData: "delete|" + bookmark.Id},
					{Text: "📄 Summary", CallbackData: "summary|" + bookmark.Id},
				},
			},
		},
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	})
}

func searchBookmarks(ctx context.Context, b *bot.Bot, chatID int64, query string) {
	req, err := http.NewRequest("GET", apiEndpoint+"/api/v1/bookmarks/search?query="+urlQueryEscape(query), nil)
	if err != nil {
		logging.Logger.Errorw("failed to create search request", "error", err, "query", query, "chatID", chatID)
		return
	}
	req.Header.Set("Authorization", "Bearer "+userAPITokens[chatID])

	resp, err := httpClient.Do(req)
	if err != nil {
		logging.Logger.Errorw("failed to send request", "error", err, "query", query, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ <b>Search failed</b>\n\nNetwork error: " + err.Error(),
			ParseMode: models.ParseModeHTML,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Logger.Errorw("failed to search bookmarks", "status", resp.Status, "query", query, "chatID", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      "❌ <b>Search failed</b>\n\nServer error: " + resp.Status,
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Logger.Errorw("failed to decode response", "error", err, "query", query, "chatID", chatID)
		return
	}

	if len(result.Bookmarks) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chatID,
			Text:      fmt.Sprintf("🔍 <b>No results found</b>\n\nNo bookmarks match <i>\"%s\"</i>\n\nTry:\n• Different keywords\n• Broader search terms\n• Check spelling", query),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 <b>Search Results</b> (%d found)\n\n", len(result.Bookmarks)))

	for i, r := range result.Bookmarks {
		if i >= 10 { // Limit to 10 results to avoid long messages
			break
		}
		sb.WriteString(fmt.Sprintf("<b>%d.</b> <a href=\"%s\">%s</a>\n", i+1, r.Link, r.Title))
		if r.Headline != "" {
			// Truncate headline if too long
			headline := r.Headline
			if len(headline) > 200 {
				headline = headline[:197] + "..."
			}
			sb.WriteString(fmt.Sprintf("<i>%s</i>\n", headline))
		}
		sb.WriteString("\n")
	}

	if len(result.Bookmarks) > 10 {
		sb.WriteString(fmt.Sprintf("<i>... and %d more results</i>", len(result.Bookmarks)-10))
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	})
}

func handleCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId := update.CallbackQuery.From.ID
	if !isUserAuthenticated(userId) {
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Please connect your account first",
			ShowAlert:       true,
		})
		return
	}

	data := update.CallbackQuery.Data
	parts := strings.Split(data, "|")
	if len(parts) != 2 {
		logging.Logger.Errorw("invalid callback data", "data", data)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Invalid action",
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
	logging.Logger.Debugw("Deleting bookmark", "id", bookmarkID, "userId", userId)
	req, _ := http.NewRequest("DELETE", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPITokens[userId])

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		logging.Logger.Errorw("failed to delete bookmark", "error", err, "status", resp.StatusCode)
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
		Text:      "🗑 <b>Deleted</b>\n\nBookmark has been removed from your library.",
		ParseMode: models.ParseModeHTML,
	})
	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "✅ Bookmark deleted",
	})
}

func getSummary(ctx context.Context, b *bot.Bot, update *models.Update, bookmarkID string) {
	userId := update.CallbackQuery.From.ID
	logging.Logger.Debugw("Getting bookmark summary", "id", bookmarkID, "userId", userId)
	req, _ := http.NewRequest("GET", apiEndpoint+"/api/v1/bookmarks/"+bookmarkID, nil)
	req.Header.Set("Authorization", "Bearer "+userAPITokens[userId])

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		logging.Logger.Errorw("failed to get bookmark summary", "error", err, "status", resp.StatusCode, "ID", bookmarkID)
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
		logging.Logger.Errorw("failed to decode bookmark", "error", err, "ID", bookmarkID)
		b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Failed to get summary",
			ShowAlert:       true,
		})
		return
	}

	logging.Logger.Debugw("Got bookmark summary", "id", bookmarkID, "title", bookmark.Title)

	var summaryText strings.Builder
	summaryText.WriteString("📄 <b>Summary</b>\n\n")
	summaryText.WriteString(fmt.Sprintf("<b>%s</b>\n", bookmark.Title))
	summaryText.WriteString(fmt.Sprintf("<a href=\"%s\">🔗 View original</a>\n\n", bookmark.Link))

	if bookmark.Excerpt != "" {
		excerpt := bookmark.Excerpt
		if len(excerpt) > 500 {
			excerpt = excerpt[:497] + "..."
		}
		summaryText.WriteString(fmt.Sprintf("<i>%s</i>", excerpt))
	} else {
		summaryText.WriteString("<i>No summary available for this bookmark.</i>")
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		Text:      summaryText.String(),
		ParseMode: models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: bot.True(),
		},
	})

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            "✅ Summary loaded",
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
