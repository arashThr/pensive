package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/models"
)

const (
	PodcastDays         = 7  // Look back 7 days for bookmarks
	PodcastArticleLimit = 10 // Max 10 articles per podcast
)

type Podcast struct {
	BookmarkModel *models.BookmarkRepo
	TTSServiceURL string
}

type PodcastResponse struct {
	Message  string `json:"message"`
	Articles int    `json:"articles"`
	Filename string `json:"filename"`
}

type ttsRequest struct {
	Text     string `json:"text"`
	Filename string `json:"filename"`
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
		writeErrorResponse(w, http.StatusNotFound, ErrorResponse{
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

	// Send request to TTS service
	logger.Infow("[podcast] Sending request to TTS service", "url", p.TTSServiceURL, "filename", filename)
	err = p.sendTTSRequest(text, filename)
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
func (p *Podcast) sendTTSRequest(text, filename string) error {
	reqBody := ttsRequest{Text: text, Filename: filename}
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
