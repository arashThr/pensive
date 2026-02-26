package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	PodcastDays         = 7  // Look back 7 days for bookmarks
	PodcastArticleLimit = 10 // Max 10 articles per podcast
	PodcastUploadDir    = "uploads/podcasts"
	gcpTTSEndpoint      = "https://texttospeech.googleapis.com/v1/text:synthesize"
)

type Podcast struct {
	BookmarkModel      *models.BookmarkRepo
	TelegramRepo       *models.TelegramRepo
	GCPProjectID       string
	ServiceAccountPath string // path to service-account.json; used in prod
	TelegramToken      string
	Environment        string // "prod" uses service account; anything else uses ADC
}

type PodcastResponse struct {
	Message  string `json:"message"`
	Articles int    `json:"articles"`
}

// GeneratePodcast fetches recent bookmarks and dispatches audio generation in a
// background goroutine, returning immediately to the client.
func (p *Podcast) GeneratePodcast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	logger.Infow("[podcast] Starting podcast generation", "userId", user.ID)

	if p.GCPProjectID == "" || p.ServiceAccountPath == "" {
		logger.Warn("[podcast] Google TTS not configured properly")
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "TTS_NOT_CONFIGURED",
			Message: "Podcast generation is not configured properly. Please contact support.",
		})
		return
	}

	bookmarks, err := p.BookmarkModel.GetRecentRandomByUserId(user.ID, PodcastDays, PodcastArticleLimit)
	if err != nil {
		logger.Errorw("[podcast] Failed to fetch bookmarks", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "FETCH_BOOKMARKS_FAILED",
			Message: "Failed to fetch recent bookmarks",
		})
		return
	}

	if len(bookmarks) == 0 {
		logger.Infow("[podcast] No bookmarks found for user", "userId", user.ID)
		writeErrorResponse(w, http.StatusNoContent, ErrorResponse{
			Code:    "NO_BOOKMARKS",
			Message: fmt.Sprintf("No bookmarks found in the past %d days", PodcastDays),
		})
		return
	}

	logger.Infow("[podcast] Found bookmarks", "count", len(bookmarks))

	text := formatPodcastText(bookmarks)
	filename := fmt.Sprintf("%v_%d.ogg", user.ID, time.Now().Unix())

	logger.Infow("[podcast] Dispatching generation goroutine", "userId", user.ID, "filename", filename)
	go p.generateAndSend(int64(user.ID), text, filename)

	response := PodcastResponse{
		Message:  fmt.Sprintf("Podcast generation started for %d articles.", len(bookmarks)),
		Articles: len(bookmarks),
	}
	if err := writeResponse(w, response); err != nil {
		logger.Errorw("[podcast] Failed to write response", "error", err)
	}
}

// generateAndSend calls Google TTS, saves the audio file, and delivers it via Telegram.
// Runs in a goroutine so it does not block the HTTP handler.
func (p *Podcast) generateAndSend(userID int64, text, filename string) {
	logger := logging.Logger

	logger.Infow("[podcast] Calling Google TTS", "userId", userID)
	audioBytes, err := p.callGoogleTTS(context.Background(), text)
	if err != nil {
		logger.Errorw("[podcast] Google TTS failed", "error", err, "userId", userID)
		return
	}

	if err := os.MkdirAll(PodcastUploadDir, 0755); err != nil {
		logger.Errorw("[podcast] Failed to create upload dir", "error", err)
		return
	}

	filePath := fmt.Sprintf("%s/%s", PodcastUploadDir, filename)
	if err := os.WriteFile(filePath, audioBytes, 0644); err != nil {
		logger.Errorw("[podcast] Failed to write audio file", "error", err, "path", filePath)
		return
	}

	logger.Infow("[podcast] Audio file saved", "path", filePath, "bytes", len(audioBytes))

	p.sendTelegramAudio(userID, filePath, filename)
}

// callGoogleTTS calls the Google Cloud Text-to-Speech API and returns raw OGG Opus bytes.
// Uses Application Default Credentials locally and a service account JSON in prod.
func (p *Podcast) callGoogleTTS(ctx context.Context, text string) ([]byte, error) {
	var httpClient *http.Client

	if p.Environment == "prod" && p.ServiceAccountPath != "" {
		credsJSON, err := os.ReadFile(p.ServiceAccountPath)
		if err != nil {
			return nil, fmt.Errorf("read service account: %w", err)
		}
		creds, err := google.CredentialsFromJSON(ctx, credsJSON, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("parse service account credentials: %w", err)
		}
		httpClient = oauth2.NewClient(ctx, creds.TokenSource)
	} else {
		creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("find default credentials (ADC): %w", err)
		}
		httpClient = oauth2.NewClient(ctx, creds.TokenSource)
	}

	reqBody := map[string]interface{}{
		"input": map[string]string{
			"prompt": "Say the following in a friendly and informative way. Put yourself in position of a podcast host.",
			"text":   text,
		},
		"voice": map[string]interface{}{
			"languageCode": "en-us",
			"name":         "Achird",
			"model_name":   "gemini-2.5-flash-tts",
		},
		"audioConfig": map[string]interface{}{
			"audioEncoding":   "OGG_OPUS",
			"sampleRateHertz": 24000,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gcpTTSEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create TTS request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-user-project", p.GCPProjectID)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call TTS API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API returned %s: %s", resp.Status, body)
	}

	var result struct {
		AudioContent string `json:"audioContent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode TTS response: %w", err)
	}

	audioBytes, err := base64.StdEncoding.DecodeString(result.AudioContent)
	if err != nil {
		return nil, fmt.Errorf("decode base64 audio: %w", err)
	}

	return audioBytes, nil
}

// sendTelegramAudio uploads the generated audio file to the user's Telegram chat.
func (p *Podcast) sendTelegramAudio(userID int64, filePath, filename string) {
	logger := logging.Logger

	if p.TelegramRepo == nil || p.TelegramToken == "" {
		logger.Infow("[podcast] Telegram not configured, skipping", "userId", userID)
		return
	}

	chatID, err := p.TelegramRepo.GetChatIdByUserId(types.UserId(userID))
	if err != nil {
		logger.Infow("[podcast] User has no Telegram linked", "userId", userID)
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		logger.Errorw("[podcast] Failed to open audio file", "error", err, "path", filePath)
		return
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	_ = mw.WriteField("caption", "🎧 Your Pensive podcast is ready!")
	part, err := mw.CreateFormFile("audio", filename)
	if err != nil {
		logger.Errorw("[podcast] Failed to create multipart form file", "error", err)
		return
	}
	if _, err := io.Copy(part, f); err != nil {
		logger.Errorw("[podcast] Failed to copy audio into form", "error", err)
		return
	}
	mw.Close()

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendAudio", p.TelegramToken)
	resp, err := http.Post(endpoint, mw.FormDataContentType(), &buf)
	if err != nil {
		logger.Errorw("[podcast] Failed to send Telegram audio", "error", err, "userId", userID)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Errorw("[podcast] Telegram API returned error", "status", resp.StatusCode, "body", string(body), "userId", userID)
		return
	}

	logger.Infow("[podcast] Sent Telegram audio", "userId", userID, "chatId", chatID)
}

// formatPodcastText creates article summaries formatted for TTS narration.
func formatPodcastText(bookmarks []models.Bookmark) string {
	var buf bytes.Buffer

	buf.WriteString("Welcome to your weekly Pensive podcast. Here are your saved articles.\n\n")

	for i, b := range bookmarks {
		buf.WriteString(fmt.Sprintf("Article %d: %s.\n\n", i+1, *b.AIExcerpt))
	}

	buf.WriteString("That's all for this week. Happy reading!")

	return buf.String()
}
