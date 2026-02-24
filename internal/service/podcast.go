package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
)

const (
	PodcastDays         = 7  // Look back 7 days for bookmarks
	PodcastArticleLimit = 10 // Max 10 articles per podcast
	PodcastUploadDir    = "uploads/podcasts"
)

type Podcast struct {
	BookmarkModel *models.BookmarkRepo
	EmailService  *EmailService
	TelegramRepo  *models.TelegramRepo
	TTSServiceURL string
	TelegramToken string
	Domain        string
}

type PodcastResponse struct {
	Message  string `json:"message"`
	Articles int    `json:"articles"`
	Filename string `json:"filename"`
}

type ttsRequest struct {
	Text        string `json:"text"`
	Filename    string `json:"filename"`
	UserEmail   string `json:"user_email"`
	UserID      int64  `json:"user_id"`
	CallbackURL string `json:"callback_url"`
}

type podcastCompleteRequest struct {
	Filename  string `json:"filename"`
	UserEmail string `json:"user_email"`
	UserID    int64  `json:"user_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// GeneratePodcast initiates podcast generation from recent bookmarks
// Sends request to TTS service which generates async, then returns response to client.
func (p *Podcast) GeneratePodcast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	logger.Infow("[podcast] Starting podcast generation", "userId", user.ID)

	// Fetch recent bookmarks
	logger.Infow("[podcast] Fetching recent bookmarks", "days", PodcastDays, "limit", PodcastArticleLimit)
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

	// Format text for TTS (numbered articles)
	text := formatPodcastText(bookmarks)
	logger.Infow("[podcast] Formatted podcast text", "charCount", len(text))

	// Generate filename
	filename := fmt.Sprintf("%v_%d.wav", user.ID, time.Now().Unix())

	// Build callback URL for TTS service to notify when complete
	callbackURL := fmt.Sprintf("%s/internal/podcast/complete", p.Domain)

	// Send request to TTS service
	logger.Infow("[podcast] Sending request to TTS service", "url", p.TTSServiceURL, "filename", filename)
	err = p.sendTTSRequest(text, filename, user.Email, int64(user.ID), callbackURL)
	if err != nil {
		logger.Errorw("[podcast] TTS service call failed", "error", err)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "TTS_FAILED",
			Message: "Failed to initiate audio generation",
		})
		return
	}

	logger.Infow("[podcast] TTS service accepted request", "filename", filename)

	// Return response to client
	response := PodcastResponse{
		Message:  fmt.Sprintf("Podcast generation started for %d articles. File will be saved as %s", len(bookmarks), filename),
		Articles: len(bookmarks),
		Filename: filename,
	}
	if err := writeResponse(w, response); err != nil {
		logger.Errorw("[podcast] Failed to write response", "error", err)
	}
}

// formatPodcastText creates a numbered list of article titles for TTS
func formatPodcastText(bookmarks []models.Bookmark) string {
	var buf bytes.Buffer

	buf.WriteString("Welcome to your weekly Pensive podcast. Here are your saved articles.\n\n")

	for i, b := range bookmarks {
		buf.WriteString(fmt.Sprintf("Article %d: %s.\n\n", i+1, *b.AIExcerpt))
	}

	buf.WriteString("That's all for this week. Happy reading!")

	return buf.String()
}

// sendTTSRequest sends text to the TTS service which saves the audio file
func (p *Podcast) sendTTSRequest(text, filename, userEmail string, userID int64, callbackURL string) error {
	reqBody := ttsRequest{
		Text:        text,
		Filename:    filename,
		UserEmail:   userEmail,
		UserID:      userID,
		CallbackURL: callbackURL,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal TTS request: %w", err)
	}

	url := fmt.Sprintf("%s/generate", p.TTSServiceURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("call TTS service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TTS service returned status %d", resp.StatusCode)
	}

	return nil
}

// PodcastComplete handles the callback from TTS service when audio generation is complete
func (p *Podcast) PodcastComplete(w http.ResponseWriter, r *http.Request) {
	logger := logging.Logger

	var req podcastCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Errorw("[podcast] Failed to decode complete request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	logger.Infow("[podcast] Received completion callback", "filename", req.Filename, "email", req.UserEmail, "userId", req.UserID, "success", req.Success)

	if !req.Success {
		logger.Errorw("[podcast] TTS generation failed", "filename", req.Filename, "error", req.Error)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Build download URL
	downloadURL := fmt.Sprintf("%s/%s/%s", p.Domain, PodcastUploadDir, req.Filename)

	// Send email with download link
	err := p.EmailService.SendPodcastReady(req.UserEmail, downloadURL)
	if err != nil {
		logger.Errorw("[podcast] Failed to send podcast email", "error", err, "email", req.UserEmail)
		// Continue to try Telegram even if email fails
	} else {
		logger.Infow("[podcast] Sent podcast ready email", "email", req.UserEmail)
	}

	// Send Telegram notification if user has linked their account
	p.sendTelegramNotification(req.UserID, downloadURL)

	w.WriteHeader(http.StatusOK)
}

// sendTelegramNotification sends podcast ready message to user's Telegram
func (p *Podcast) sendTelegramNotification(userID int64, downloadURL string) {
	if p.TelegramRepo == nil || p.TelegramToken == "" {
		return
	}

	chatID, err := p.TelegramRepo.GetChatIdByUserId(types.UserId(userID))
	if err != nil {
		logging.Logger.Infow("[podcast] User has no Telegram linked", "userId", userID)
		return
	}

	message := fmt.Sprintf("🎧 Your Pensive podcast is ready!\n\nDownload here: %s", downloadURL)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.TelegramToken)
	body := fmt.Sprintf(`{"chat_id": %d, "text": %q}`, chatID, message)

	resp, err := http.Post(endpoint, "application/json", bytes.NewBufferString(body))
	if err != nil {
		logging.Logger.Errorw("[podcast] Failed to send Telegram message", "error", err, "userId", userID)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Logger.Errorw("[podcast] Telegram API returned error", "status", resp.StatusCode, "userId", userID)
		return
	}

	logging.Logger.Infow("[podcast] Sent Telegram notification", "userId", userID, "chatId", chatID)
}
