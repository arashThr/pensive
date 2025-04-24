package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	telebot "gopkg.in/telebot.v4"
)

type Bot struct {
	bot       *telebot.Bot
	apiClient *APIClient
}

type APIClient struct {
	endpoint string
	token    string
	client   *http.Client
}

type BookmarkResponse struct {
	BookmarkId    string
	Title         string
	Link          string
	Excerpt       string
	CreatedAt     time.Time
	PublishedTime *time.Time
}

type SearchResult struct {
	Headline   string
	BookmarkId string
	Title      string
	Link       string
	Excerpt    string
	Rank       float32
}

type SearchResponse struct {
	Bookmarks []SearchResult
}

func NewBot(token, apiEndpoint string) (*Bot, error) {
	bot, err := telebot.NewBot(telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10},
	})
	if err != nil {
		return nil, err
	}

	return &Bot{
		bot: bot,
		apiClient: &APIClient{
			endpoint: apiEndpoint,
			client:   &http.Client{},
		},
	}, nil
}

func (b *Bot) Start() {
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle(telebot.OnText, b.handleText)
	b.bot.Handle(telebot.OnCallback, b.handleCallback)

	b.bot.Start()
}

func (b *Bot) handleStart(c telebot.Context) error {
	return c.Send("Please provide your API token from https://yourwebsite.com/account")
}

func (b *Bot) handleText(c telebot.Context) error {
	text := c.Text()

	// Check if the user has set their token
	if b.apiClient.token == "" {
		b.apiClient.token = strings.TrimSpace(text)
		c.Delete()
		return c.Edit("Token set! You can now send links to bookmark or text to search.")
	}

	// Check if the text is a URL
	if isURL(text) {
		return b.handleBookmark(c, text)
	}

	// Otherwise, treat as search query
	return b.handleSearch(c, text)
}

func (b *Bot) handleBookmark(c telebot.Context, link string) error {
	bookmark, err := b.apiClient.CreateBookmark(link)
	if err != nil {
		return c.Send("Failed to save bookmark: " + err.Error())
	}
	fmt.Printf("%+v\n", bookmark)

	// Create inline keyboard with Delete and Summary buttons
	selector := &telebot.ReplyMarkup{}
	deleteBtn := selector.Data("Delete", "delete|"+bookmark.BookmarkId)
	summaryBtn := selector.Data("Summary", "summary|"+bookmark.BookmarkId)
	selector.Inline(selector.Row(deleteBtn, summaryBtn))

	return c.Send(
		fmt.Sprintf("Bookmark saved: %s", bookmark.Title),
		selector,
	)
}

func (b *Bot) handleSearch(c telebot.Context, query string) error {
	results, err := b.apiClient.SearchBookmarks(query)
	if err != nil {
		return c.Send("Search failed: " + err.Error())
	}

	if len(results) == 0 {
		return c.Send("No results found.")
	}

	var response strings.Builder
	for i, bookmark := range results {
		response.WriteString(fmt.Sprintf("%d. %s\n%s\n%s\n\n", i+1, bookmark.Title, bookmark.Link, bookmark.Headline))
	}

	return c.Send(response.String())
}

func (b *Bot) handleCallback(c telebot.Context) error {
	data := strings.TrimSpace(c.Callback().Data)
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "Invalid callback"})
	}

	fmt.Println("parts: ", parts)
	action, id := parts[0], parts[1]

	switch action {
	case "delete":
		err := b.apiClient.DeleteBookmark(id)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "Failed to delete bookmark"})
		}
		err = c.Edit("Bookmark deleted")
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "Failed to update message"})
		}
		return c.Respond(&telebot.CallbackResponse{})

	case "summary":
		bookmark, err := b.apiClient.GetBookmark(id)
		fmt.Println("Summary", bookmark, id)
		if err != nil {
			slog.Error("getting bookmark", "error", err)
			return c.Respond(&telebot.CallbackResponse{Text: "Failed to get summary"})
		}
		return c.Respond(&telebot.CallbackResponse{Text: bookmark.Excerpt})
	}

	return c.Respond(&telebot.CallbackResponse{Text: "Unknown action"})
}

func (ac *APIClient) CreateBookmark(link string) (*BookmarkResponse, error) {
	body, _ := json.Marshal(map[string]string{"link": link})
	req, err := http.NewRequest("POST", ac.endpoint+"/api/v1/bookmarks", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ac.token)

	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var bookmark BookmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil {
		return nil, err
	}
	return &bookmark, nil
}

func (ac *APIClient) DeleteBookmark(id string) error {
	req, err := http.NewRequest("DELETE", ac.endpoint+"/api/v1/bookmarks/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+ac.token)

	resp, err := ac.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}
	return nil
}

func (ac *APIClient) GetBookmark(id string) (*BookmarkResponse, error) {
	req, err := http.NewRequest("GET", ac.endpoint+"/api/v1/bookmarks/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ac.token)

	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var bookmark BookmarkResponse
	if err := json.NewDecoder(resp.Body).Decode(&bookmark); err != nil {
		return nil, err
	}
	return &bookmark, nil
}

func (ac *APIClient) SearchBookmarks(query string) ([]SearchResult, error) {
	req, err := http.NewRequest("GET", ac.endpoint+"/api/v1/bookmarks/search?query="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ac.token)

	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}
	return searchResp.Bookmarks, nil
}

func isURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

func main() {
	// telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramToken := "365677088:AAHfGI47u4v5PcQK-yDzxUV_htFOvQzEM58"
	if telegramToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	// apiEndpoint := os.Getenv("API_ENDPOINT")
	apiEndpoint := "http://localhost:8000"
	if apiEndpoint == "" {
		log.Fatal("API_ENDPOINT is not set")
	}

	bot, err := NewBot(telegramToken, apiEndpoint)
	if err != nil {
		log.Fatal("Failed to create bot: ", err)
	}

	log.Println("Starting Telegram bot...")
	bot.Start()
}
