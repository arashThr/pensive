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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
)

const (
	PodcastDays         = 7  // Look back 7 days for bookmarks (weekly)
	DailyPodcastDays    = 1  // Look back 1 day for bookmarks (daily)
	PodcastArticleLimit = 10 // Max 10 articles per podcast
	PodcastUploadDir    = "uploads/podcasts"
	PodcastSummaryDir   = "uploads/podcasts/summary"
	gcpTTSEndpoint      = "https://texttospeech.googleapis.com/v1/text:synthesize"
	gcpTTSTimeout       = 10 * time.Minute // generous timeout; TTS can be slow for long texts

	// TODO: Increase it to an hour when testing it done
	podcastSchedulerInterval = time.Minute
)

// userPodcastDir returns the upload directory for a specific user's podcast episodes.
func userPodcastDir(userID int64) string {
	return fmt.Sprintf("%s/%d", PodcastSummaryDir, userID)
}

// weekdayNumbers maps lowercase day names to time.Weekday values.
var weekdayNumbers = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

type Podcast struct {
	BookmarkModel       *models.BookmarkRepo
	TelegramRepo        *models.TelegramRepo
	PodcastScheduleRepo *models.PodcastScheduleRepo
	UserRepo            *models.UserRepo
	EmailService        *EmailService
	GenAIClient         *genai.Client
	GCPProjectID        string
	ServiceAccountPath  string // path to service-account.json; used in prod
	TelegramToken       string
	Environment         string // "prod" uses service account; anything else uses ADC
	Domain              string
}

// ServeEpisode serves a podcast audio file to its authenticated owner.
// URL: GET /podcast/episodes/{filename}
func (p *Podcast) ServeEpisode(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	filename := chi.URLParam(r, "filename")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	// Restrict to safe filenames (no path traversal).
	if strings.ContainsAny(filename, "/\\") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	filePath := fmt.Sprintf("%s/%d/%s", PodcastSummaryDir, user.ID, filename)
	if _, err := os.Stat(filePath); err != nil {
		logger.Infow("[podcast] Episode file not found", "userId", user.ID, "filename", filename)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "audio/ogg")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	http.ServeFile(w, r, filePath)
}

// ---- Scheduler ---------------------------------------------------------------

// StartScheduler launches the hourly background job that processes due podcast
// schedules. It blocks until ctx is cancelled – call it in a goroutine.
func (p *Podcast) StartScheduler(ctx context.Context) {
	logging.Logger.Infow("[podcast-scheduler] Starting")

	// Run once immediately on startup to catch anything that was missed.
	p.runSchedulerTick(ctx)

	ticker := time.NewTicker(podcastSchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Infow("[podcast-scheduler] Stopping")
			return
		case <-ticker.C:
			p.runSchedulerTick(ctx)
		}
	}
}

func (p *Podcast) runSchedulerTick(ctx context.Context) {
	logger := logging.Logger

	// Reap any schedules stuck in 'processing' for too long.
	reaped, err := p.PodcastScheduleRepo.ReapTimedOut()
	if err != nil {
		logger.Errorw("[podcast-scheduler] ReapTimedOut failed", "error", err)
	} else if reaped > 0 {
		logger.Infow("[podcast-scheduler] Reaped stale schedules", "count", reaped)
	}

	due, err := p.PodcastScheduleRepo.GetDue(models.PodcastScheduleTypeWeekly)
	if err != nil {
		logger.Errorw("[podcast-scheduler] GetDue failed", "error", err)
		return
	}

	if len(due) == 0 {
		return
	}

	logger.Infow("[podcast-scheduler] Dispatching due episodes", "count", len(due))

	for _, schedule := range due {
		// Atomically claim the row before spawning the goroutine to avoid
		// double-processing in case of concurrent scheduler instances.
		if err := p.PodcastScheduleRepo.MarkProcessing(schedule.ID); err != nil {
			if errors.Is(err, errors.ErrNotFound) {
				continue // already claimed
			}
			logger.Errorw("[podcast-scheduler] MarkProcessing failed", "error", err, "scheduleId", schedule.ID)
			continue
		}
		s := schedule // capture loop variable - We're go 1.22+, so just to be sure :)
		go p.processSchedule(ctx, s)
	}
}

// processSchedule generates and delivers one scheduled episode, then updates the DB.
func (p *Podcast) processSchedule(ctx context.Context, s models.PodcastSchedule) {
	logger := logging.Logger
	logger.Infow("[podcast-scheduler] Processing episode", "scheduleId", s.ID, "userId", s.UserID)

	fail := func(err error) {
		logger.Errorw("[podcast-scheduler] Episode failed", "error", err, "scheduleId", s.ID)
		if dbErr := p.PodcastScheduleRepo.MarkFailed(s.ID); dbErr != nil {
			logger.Errorw("[podcast-scheduler] MarkFailed error", "error", dbErr)
		}
	}

	prefs, err := p.UserRepo.GetSummaryPreferences(s.UserID)
	if err != nil {
		fail(fmt.Errorf("get podcast summary preferences: %w", err))
		return
	}

	articles, err := p.BookmarkModel.GetRecentRandomForPodcast(s.UserID, PodcastDays, PodcastArticleLimit)
	if err != nil {
		fail(fmt.Errorf("fetch bookmarks: %w", err))
		return
	}

	if len(articles) == 0 {
		logger.Infow("[podcast-scheduler] No bookmarks, skipping and rescheduling", "userId", s.UserID)
		next := NextPublishAt(prefs.Day, 7)
		if dbErr := p.PodcastScheduleRepo.MarkSent(s.ID, next); dbErr != nil {
			logger.Errorw("[podcast-scheduler] MarkSent (no bookmarks) error", "error", dbErr)
		}
		return
	}

	script, err := p.generatePodcastScript(ctx, s.UserID, articles, PodcastDays)
	if err != nil {
		fail(fmt.Errorf("generate podcast script: %w", err))
		return
	}

	ts := time.Now().Unix()
	baseFilename := fmt.Sprintf("%d", ts) // shared timestamp stem for both files

	uploadDir := userPodcastDir(int64(s.UserID))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fail(fmt.Errorf("create upload dir: %w", err))
		return
	}

	// Save the podcast script alongside the audio for reference / debugging.
	scriptPath := fmt.Sprintf("%s/%s_script.txt", uploadDir, baseFilename)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		logger.Warnw("[podcast-scheduler] Failed to write script file", "error", err, "path", scriptPath)
		// Non-fatal — continue with audio generation.
	}

	audioBytes, err := p.callGoogleTTS(ctx, script)
	if err != nil {
		fail(fmt.Errorf("google TTS: %w", err))
		return
	}

	audioFilename := fmt.Sprintf("%s.ogg", baseFilename)
	audioPath := fmt.Sprintf("%s/%s", uploadDir, audioFilename)
	if err := os.WriteFile(audioPath, audioBytes, 0644); err != nil {
		fail(fmt.Errorf("write audio file: %w", err))
		return
	}

	logger.Infow("[podcast-scheduler] Audio saved", "path", audioPath, "bytes", len(audioBytes))

	sentViaTelegram := false
	if prefs.Telegram {
		sentViaTelegram = p.sendTelegramAudio(int64(s.UserID), audioPath, audioFilename)
	}

	// Fall back to email when Telegram is not enabled or sending failed (e.g. user hasn't linked Telegram).
	if !sentViaTelegram {
		user, err := p.UserRepo.Get(s.UserID)
		if err != nil {
			logger.Errorw("[podcast-scheduler] Could not look up user email for podcast notification", "error", err, "userId", s.UserID)
		} else {
			p.sendPodcastEmail(user.Email, int64(s.UserID), audioFilename)
		}
	}

	next := NextPublishAt(prefs.Day, 7)
	if dbErr := p.PodcastScheduleRepo.MarkSent(s.ID, next); dbErr != nil {
		logger.Errorw("[podcast-scheduler] MarkSent error", "error", dbErr, "scheduleId", s.ID)
	}

	logger.Infow("[podcast-scheduler] Episode complete, next scheduled", "userId", s.UserID, "nextAt", next)
}

// ---- Daily scheduler --------------------------------------------------------

// StartDailyScheduler runs the hourly tick that processes daily podcast schedules.
func (p *Podcast) StartDailyScheduler(ctx context.Context) {
	logging.Logger.Infow("[podcast-daily] Starting")
	p.runDailySchedulerTick(ctx)

	ticker := time.NewTicker(podcastSchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Logger.Infow("[podcast-daily] Stopping")
			return
		case <-ticker.C:
			p.runDailySchedulerTick(ctx)
		}
	}
}

func (p *Podcast) runDailySchedulerTick(ctx context.Context) {
	logger := logging.Logger

	// ReapTimedOut covers all schedule types in one pass; only run it here to
	// avoid double-reaping when both scheduler ticks run close together.
	due, err := p.PodcastScheduleRepo.GetDue(models.PodcastScheduleTypeDaily)
	if err != nil {
		logger.Errorw("[podcast-daily] GetDue failed", "error", err)
		return
	}
	for _, schedule := range due {
		if err := p.PodcastScheduleRepo.MarkProcessing(schedule.ID); err != nil {
			if errors.Is(err, errors.ErrNotFound) {
				continue
			}
			logger.Errorw("[podcast-daily] MarkProcessing failed", "error", err, "scheduleId", schedule.ID)
			continue
		}
		s := schedule
		go p.processDailySchedule(ctx, s)
	}
}

func (p *Podcast) processDailySchedule(ctx context.Context, s models.PodcastSchedule) {
	logger := logging.Logger
	logger.Infow("[podcast-daily] Processing episode", "scheduleId", s.ID, "userId", s.UserID)

	fail := func(err error) {
		logger.Errorw("[podcast-daily] Episode failed", "error", err, "scheduleId", s.ID)
		if dbErr := p.PodcastScheduleRepo.MarkFailed(s.ID); dbErr != nil {
			logger.Errorw("[podcast-daily] MarkFailed error", "error", dbErr)
		}
	}

	prefs, err := p.UserRepo.GetSummaryPreferences(s.UserID)
	if err != nil {
		fail(fmt.Errorf("get preferences: %w", err))
		return
	}

	articles, err := p.BookmarkModel.GetRecentRandomForPodcast(s.UserID, DailyPodcastDays, PodcastArticleLimit)
	if err != nil {
		fail(fmt.Errorf("fetch bookmarks: %w", err))
		return
	}

	// Reschedule and skip silently if there are no bookmarks today.
	if len(articles) == 0 {
		logger.Infow("[podcast-daily] No bookmarks today, rescheduling", "userId", s.UserID)
		next := NextDailyFireAt(prefs.DailyHour, prefs.DailyTimezone)
		if dbErr := p.PodcastScheduleRepo.MarkSent(s.ID, next); dbErr != nil {
			logger.Errorw("[podcast-daily] MarkSent (no bookmarks) error", "error", dbErr)
		}
		return
	}

	script, err := p.generatePodcastScript(ctx, s.UserID, articles, DailyPodcastDays)
	if err != nil {
		fail(fmt.Errorf("generate podcast script: %w", err))
		return
	}

	ts := time.Now().Unix()
	baseFilename := fmt.Sprintf("%d", ts)
	uploadDir := userPodcastDir(int64(s.UserID))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fail(fmt.Errorf("create upload dir: %w", err))
		return
	}

	scriptPath := fmt.Sprintf("%s/%s_script.txt", uploadDir, baseFilename)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		logger.Warnw("[podcast-daily] Failed to write script file", "error", err, "path", scriptPath)
	}

	audioBytes, err := p.callGoogleTTS(ctx, script)
	if err != nil {
		fail(fmt.Errorf("google TTS: %w", err))
		return
	}

	audioFilename := fmt.Sprintf("%s.ogg", baseFilename)
	audioPath := fmt.Sprintf("%s/%s", uploadDir, audioFilename)
	if err := os.WriteFile(audioPath, audioBytes, 0644); err != nil {
		fail(fmt.Errorf("write audio file: %w", err))
		return
	}

	logger.Infow("[podcast-daily] Audio saved", "path", audioPath, "bytes", len(audioBytes))

	// Daily podcast is Telegram-only.
	if !p.sendTelegramAudio(int64(s.UserID), audioPath, audioFilename) {
		logger.Warnw("[podcast-daily] Telegram send failed or not linked", "userId", s.UserID)
	}

	next := NextDailyFireAt(prefs.DailyHour, prefs.DailyTimezone)
	if dbErr := p.PodcastScheduleRepo.MarkSent(s.ID, next); dbErr != nil {
		logger.Errorw("[podcast-daily] MarkSent error", "error", dbErr, "scheduleId", s.ID)
	}
	logger.Infow("[podcast-daily] Episode complete, next scheduled", "userId", s.UserID, "nextAt", next)
}

// ---- Generation helpers -------------------------------------------------------

// ttsChunkMaxBytes is the conservative byte limit per TTS request text field.
// The API hard-limits at 4000; we use 3500 for a comfortable margin.
const ttsChunkMaxBytes = 3500

// callGoogleTTS splits the script into ≤3500-byte chunks, synthesises each one
// sequentially, then concatenates the OGG Opus files via ffmpeg.
func (p *Podcast) callGoogleTTS(ctx context.Context, text string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gcpTTSTimeout)
	defer cancel()

	start := time.Now()
	defer func() {
		logging.Logger.Infow("[podcast] Google TTS completed",
			"elapsed", time.Since(start).Round(time.Millisecond).String())
	}()

	httpClient, err := p.buildGCPHTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	chunks := splitTextIntoChunks(text, ttsChunkMaxBytes)
	logging.Logger.Infow("[podcast] TTS chunks", "count", len(chunks))

	if len(chunks) == 1 {
		return p.callGoogleTTSChunk(ctx, httpClient, chunks[0])
	}

	// Multi-chunk: synthesise each part, then stitch with ffmpeg.
	tmpDir, err := os.MkdirTemp("", "podcast-tts-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var chunkPaths []string
	for i, chunk := range chunks {
		audio, err := p.callGoogleTTSChunk(ctx, httpClient, chunk)
		if err != nil {
			return nil, fmt.Errorf("TTS chunk %d: %w", i, err)
		}
		path := filepath.Join(tmpDir, fmt.Sprintf("chunk_%03d.ogg", i))
		if err := os.WriteFile(path, audio, 0644); err != nil {
			return nil, fmt.Errorf("write TTS chunk %d: %w", i, err)
		}
		chunkPaths = append(chunkPaths, path)
	}

	// Build ffmpeg concat-list file.
	var listBuf strings.Builder
	for _, path := range chunkPaths {
		fmt.Fprintf(&listBuf, "file '%s'\n", path)
	}
	listPath := filepath.Join(tmpDir, "list.txt")
	if err := os.WriteFile(listPath, []byte(listBuf.String()), 0644); err != nil {
		return nil, fmt.Errorf("write ffmpeg list: %w", err)
	}

	outputPath := filepath.Join(tmpDir, "output.ogg")
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg concat: %w\noutput: %s", err, out)
	}

	return os.ReadFile(outputPath)
}

// buildGCPHTTPClient returns an oauth2 HTTP client authenticated for Cloud TTS.
func (p *Podcast) buildGCPHTTPClient(ctx context.Context) (*http.Client, error) {
	if p.Environment == "prod" && p.ServiceAccountPath != "" {
		credsJSON, err := os.ReadFile(p.ServiceAccountPath)
		if err != nil {
			return nil, fmt.Errorf("read service account: %w", err)
		}
		creds, err := google.CredentialsFromJSON(ctx, credsJSON, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("parse service account credentials: %w", err)
		}
		return oauth2.NewClient(ctx, creds.TokenSource), nil
	}
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("find default credentials (ADC): %w", err)
	}
	return oauth2.NewClient(ctx, creds.TokenSource), nil
}

// callGoogleTTSChunk sends a single text chunk to the TTS API and returns OGG bytes.
func (p *Podcast) callGoogleTTSChunk(ctx context.Context, httpClient *http.Client, text string) ([]byte, error) {
	const podcastHostPrompt = "Read this podcast script aloud as a warm, confident, and engaging host. " +
		"Let it be a bit messy, like someone going through series of notes and narrating their thoughts in a natural flow. " +
		"Speak naturally and conversationally — relaxed but sharp, with comfortable pacing. " +
		"Do not add, remove, or change any content; just deliver the written script as a natural podcast host would. " +
		"As for your tone, make it warm, witty, and direct. Like a smart friend catching you up over coffee. " +
		"Speak TO the listener personally"

	reqBody := map[string]interface{}{
		"input": map[string]string{
			"prompt": podcastHostPrompt,
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

// splitTextIntoChunks splits text at paragraph (\n\n) boundaries into chunks
// whose byte length does not exceed maxBytes. Paragraphs that individually exceed
// maxBytes are further split at sentence-ending punctuation ('. ', '! ', '? ').
func splitTextIntoChunks(text string, maxBytes int) []string {
	paragraphs := strings.Split(text, "\n\n")

	var chunks []string
	var cur strings.Builder

	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
	}

	addUnit := func(unit string) {
		sep := ""
		if cur.Len() > 0 {
			sep = " "
		}
		if cur.Len()+len(sep)+len(unit) > maxBytes {
			flush()
		}
		cur.WriteString(sep)
		cur.WriteString(unit)
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if len(para) <= maxBytes {
			// Paragraph fits; try to pack into current chunk.
			sep := ""
			if cur.Len() > 0 {
				sep = "\n\n"
			}
			if cur.Len()+len(sep)+len(para) > maxBytes {
				flush()
			}
			if cur.Len() > 0 {
				cur.WriteString("\n\n")
			}
			cur.WriteString(para)
		} else {
			// Paragraph too long; flush current and split at sentence boundaries.
			flush()
			for _, sentence := range splitAtSentences(para) {
				addUnit(sentence)
			}
		}
	}
	flush()

	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}

// splitAtSentences splits text at '. ', '! ', '? ' boundaries.
func splitAtSentences(text string) []string {
	var sentences []string
	remaining := text
	for len(remaining) > 0 {
		// Find the earliest sentence-ending boundary.
		cut := -1
		for _, delim := range []string{". ", "! ", "? "} {
			if i := strings.Index(remaining, delim); i >= 0 && (cut < 0 || i < cut) {
				cut = i + len(delim) - 1 // include the punctuation, exclude the trailing space
			}
		}
		if cut < 0 {
			// No more boundaries; treat remainder as one sentence.
			sentences = append(sentences, strings.TrimSpace(remaining))
			break
		}
		sentences = append(sentences, strings.TrimSpace(remaining[:cut+1]))
		remaining = strings.TrimSpace(remaining[cut+1:])
	}
	return sentences
}

// sendTelegramAudio uploads the generated audio file to the user's Telegram chat.
// Returns true if the audio was successfully sent.
func (p *Podcast) sendTelegramAudio(userID int64, filePath, filename string) bool {
	logger := logging.Logger

	if p.TelegramRepo == nil || p.TelegramToken == "" {
		logger.Infow("[podcast] Telegram not configured, skipping", "userId", userID)
		return false
	}

	chatID, err := p.TelegramRepo.GetChatIdByUserId(types.UserId(userID))
	if err != nil {
		logger.Infow("[podcast] User has no Telegram linked", "userId", userID)
		return false
	}

	f, err := os.Open(filePath)
	if err != nil {
		logger.Errorw("[podcast] Failed to open audio file", "error", err, "path", filePath)
		return false
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	_ = mw.WriteField("caption", "🎧 Your Pensive podcast is ready!")
	part, err := mw.CreateFormFile("audio", filename)
	if err != nil {
		logger.Errorw("[podcast] Failed to create multipart form file", "error", err)
		return false
	}
	if _, err := io.Copy(part, f); err != nil {
		logger.Errorw("[podcast] Failed to copy audio into form", "error", err)
		return false
	}
	mw.Close()

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendAudio", p.TelegramToken)
	resp, err := http.Post(endpoint, mw.FormDataContentType(), &buf)
	if err != nil {
		logger.Errorw("[podcast] Failed to send Telegram audio", "error", err, "userId", userID)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Errorw("[podcast] Telegram API returned error", "status", resp.StatusCode, "body", string(body), "userId", userID)
		return false
	}

	logger.Infow("[podcast] Sent Telegram audio", "userId", userID, "chatId", chatID)
	return true
}

// sendPodcastEmail sends the authenticated download link to the user's email address.
func (p *Podcast) sendPodcastEmail(userEmail string, userID int64, audioFilename string) {
	if p.EmailService == nil || p.Domain == "" {
		logging.Logger.Warnw("[podcast] Email service not configured, skipping email", "userId", userID)
		return
	}
	downloadURL := fmt.Sprintf("%s/users/podcast/episodes/%s", p.Domain, audioFilename)
	if err := p.EmailService.SendPodcastReady(userEmail, downloadURL); err != nil {
		logging.Logger.Errorw("[podcast] Failed to send podcast email", "error", err, "userId", userID, "email", userEmail)
		return
	}
	logging.Logger.Infow("[podcast] Sent podcast email", "userId", userID, "email", userEmail)
}

// TriggerEpisode is an internal admin endpoint that generates a fresh podcast episode
// for a given user and delivers it via the requested channel.
//
// POST /internal/podcast/trigger
//
//	{"user_id": 42, "channel": "email|telegram|both"}
//
// Returns 202 immediately; generation runs in a background goroutine.
func (p *Podcast) TriggerEpisode(w http.ResponseWriter, r *http.Request) {
	logger := logging.Logger

	var req struct {
		UserID  int64  `json:"user_id"`
		Channel string `json:"channel"` // "email", "telegram", "both"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if req.Channel == "" {
		req.Channel = "both"
	}
	switch req.Channel {
	case "email", "telegram", "both":
	default:
		http.Error(w, `channel must be "email", "telegram", or "both"`, http.StatusBadRequest)
		return
	}

	user, err := p.UserRepo.Get(types.UserId(req.UserID))
	if err != nil {
		logger.Errorw("[podcast-trigger] User not found", "userId", req.UserID, "error", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	logger.Infow("[podcast-trigger] Manual trigger accepted", "userId", req.UserID, "channel", req.Channel)
	go p.triggerAndDeliver(user.ID, user.Email, req.Channel)

	w.WriteHeader(http.StatusAccepted)
	_ = writeResponse(w, map[string]any{
		"message": fmt.Sprintf("episode generation started for user %d via %s", req.UserID, req.Channel),
		"user_id": req.UserID,
		"channel": req.Channel,
	})
}

// triggerAndDeliver is the backend of TriggerEpisode. It generates a fresh episode
// and delivers it over the channels requested by the caller.
func (p *Podcast) triggerAndDeliver(userID types.UserId, userEmail, channel string) {
	logger := logging.Logger

	articles, err := p.BookmarkModel.GetRecentRandomForPodcast(userID, PodcastDays, PodcastArticleLimit)
	if err != nil {
		logger.Errorw("[podcast-trigger] Failed to fetch bookmarks", "error", err, "userId", userID)
		return
	}
	if len(articles) == 0 {
		logger.Infow("[podcast-trigger] No bookmarks found, aborting", "userId", userID)
		return
	}

	script, err := p.generatePodcastScript(context.Background(), userID, articles, PodcastDays)
	if err != nil {
		logger.Errorw("[podcast-trigger] Failed to generate podcast script", "error", err, "userId", userID)
		return
	}

	uploadDir := userPodcastDir(int64(userID))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logger.Errorw("[podcast-trigger] Failed to create upload dir", "error", err)
		return
	}

	baseFilename := fmt.Sprintf("%d", time.Now().Unix())

	scriptPath := fmt.Sprintf("%s/%s_script.txt", uploadDir, baseFilename)
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		logger.Warnw("[podcast-trigger] Failed to write script file", "error", err, "path", scriptPath)
	}

	audioBytes, err := p.callGoogleTTS(context.Background(), script)
	if err != nil {
		logger.Errorw("[podcast-trigger] Google TTS failed", "error", err, "userId", userID)
		return
	}

	audioFilename := fmt.Sprintf("%s.ogg", baseFilename)
	audioPath := fmt.Sprintf("%s/%s", uploadDir, audioFilename)
	if err := os.WriteFile(audioPath, audioBytes, 0644); err != nil {
		logger.Errorw("[podcast-trigger] Failed to write audio file", "error", err, "path", audioPath)
		return
	}

	logger.Infow("[podcast-trigger] Audio saved", "path", audioPath, "bytes", len(audioBytes))

	sendEmail := channel == "email" || channel == "both"
	sendTelegram := channel == "telegram" || channel == "both"

	if sendTelegram {
		p.sendTelegramAudio(int64(userID), audioPath, audioFilename)
	}
	if sendEmail {
		p.sendPodcastEmail(userEmail, int64(userID), audioFilename)
	}

	logger.Infow("[podcast-trigger] Manual trigger complete", "userId", userID, "channel", channel)
}

// maxMarkdownCharsPerArticle caps the content per article sent to Gemini for script generation.
const maxMarkdownCharsPerArticle = 6000

// generatePodcastScript calls Gemini to write a ready-to-read podcast script (~10 min / ~1400 words).
// It also fetches all titles from the period to build the opening date + period overview.
func (p *Podcast) generatePodcastScript(ctx context.Context, userID types.UserId, articles []models.PodcastArticle, days int) (string, error) {
	if p.GenAIClient == nil {
		return "", fmt.Errorf("GenAI client not initialised")
	}

	// Fetch every title from the period for the opening overview (non-fatal if it fails).
	allTitles, _ := p.BookmarkModel.GetAllTitlesInPeriod(userID, days)

	var periodLabel string
	var targetWords int
	if days == 1 {
		periodLabel = "today"
		targetWords = 700
	} else {
		periodLabel = fmt.Sprintf("the past %d days", days)
		targetWords = 1400
	}
	targetMinutes := targetWords / 140 // ~140 wpm

	var prompt bytes.Buffer
	epDate := time.Now().UTC().Format("Monday, January 2 2006")
	fmt.Fprintf(&prompt, "You are writing the script for a personal podcast episode dated %s.\n", epDate)
	fmt.Fprintf(&prompt, "This episode covers articles the listener saved %s.\n", periodLabel)
	prompt.WriteString(`The listener uses Pensive to save articles they want to read later.
This podcast is their personal reading companion — you have read everything they saved
and you are walking them through the highlights.

== TONE & STYLE ==
- Warm, witty, and direct. Like a smart friend catching you up over coffee.
- Speak TO the listener personally: "you saved this one", "this is the one about...", "here's where it gets interesting".
- Highlight what's genuinely interesting or surprising; point out important sections by name.
- Add a line of context or "why this matters" where it genuinely helps — one thought at a time.
- Dry humour is welcome; forced jokes are not.
- No filler openers: never "Certainly!", "Absolutely!", "Great!", "Sure thing!".
- Do NOT narrate markdown syntax (#, **, -, etc.). Speak ideas, not formatting.
- Never read content verbatim. Narrate and synthesis.

`)
	fmt.Fprintf(&prompt, "== LENGTH ==\nTarget for 10 articles: approximately %d words — roughly %d minutes of listening.\n"+
		"In other words, each article suppose to take a minute or so. It's not about filling the time, "+
		"but about giving a concise and engaging narrative that respects the listener's time and attention.\n"+
		"Stop naturally after the closing; do not pad.\n\n", targetWords, targetMinutes)
	prompt.WriteString(`== STRUCTURE ==
1. OPENING (~60 words)
   Greet the listener, state today's date.
   Give a one-sentence overview: how many articles they saved and the rough themes.
   Use the full title list below for this overview, not just the featured articles.

`)

	// Full title list for the opening overview.
	if len(allTitles) > 0 {
		fmt.Fprintf(&prompt, "ALL %d ARTICLES SAVED %s (titles only — for your opening overview):\n",
			len(allTitles), strings.ToUpper(periodLabel))
		for i, t := range allTitles {
			if t.SiteName != "" {
				fmt.Fprintf(&prompt, "%d. %s (%s)\n", i+1, t.Title, t.SiteName)
			} else {
				fmt.Fprintf(&prompt, "%d. %s\n", i+1, t.Title)
			}
		}
		prompt.WriteString("\n")
	}

	// Featured articles with full content.
	fmt.Fprintf(&prompt, "2. FEATURED ARTICLES (%d articles — cover each one in depth):\n\n", len(articles))
	for i, a := range articles {
		fmt.Fprintf(&prompt, "--- Article %d ---\n", i+1)
		fmt.Fprintf(&prompt, "Title: %s\n", a.Title)
		if a.SiteName != "" {
			fmt.Fprintf(&prompt, "Source: %s\n", a.SiteName)
		}
		prompt.WriteString("\n")

		content := a.AIMarkdown
		if content == "" && a.AISummary != nil && *a.AISummary != "" {
			content = *a.AISummary
		}
		if content == "" && a.AIExcerpt != nil && *a.AIExcerpt != "" {
			content = *a.AIExcerpt
		}
		if content == "" {
			content = "(No content available — narrate based on the title alone.)"
		}
		if len(content) > maxMarkdownCharsPerArticle {
			content = content[:maxMarkdownCharsPerArticle] + "\n... [content truncated]"
		}

		prompt.WriteString(content)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString(`3. CLOSING (~40 words)
   Sign off naturally and personally. Encourage the listener to keep saving good reads.
   No cliché sign-offs like "That's all for this week" or "Thanks for listening".

Output ONLY the finished spoken script — no stage directions, markdown headers, or meta-commentary.
`)

	start := time.Now()
	result, err := p.GenAIClient.Models.GenerateContent(
		ctx,
		"gemini-3-flash-preview",
		genai.Text(prompt.String()),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("gemini script generation: %w", err)
	}
	logging.Logger.Infow("[podcast] Gemini script generation complete",
		"elapsed", time.Since(start).Round(time.Millisecond).String(),
		"userId", userID,
	)

	return strings.TrimSpace(result.Text()), nil
}

// NextDailyFireAt returns the next UTC time when the given hour occurs in the
// user's timezone. If the hour has already passed today it returns tomorrow's occurrence.
func NextDailyFireAt(hour int, timezone string) time.Time {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, loc)
	if !now.Before(target) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}

// NextPublishAt returns the next occurrence of the given weekday name that is
// at least minDays from now, at noon UTC. Defaults to Sunday on unknown input.
func NextPublishAt(day string, minDays int) time.Time {
	target, ok := weekdayNumbers[strings.ToLower(day)]
	if !ok {
		target = time.Sunday
	}

	now := time.Now().UTC()
	// Start from minDays ahead, truncated to noon UTC.
	candidate := now.AddDate(0, 0, minDays)
	candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 12, 0, 0, 0, time.UTC)

	// Advance until we land on the right weekday.
	for candidate.Weekday() != target {
		candidate = candidate.AddDate(0, 0, 1)
	}

	return candidate
}
