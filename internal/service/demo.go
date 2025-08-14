package service

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/arashthr/go-course/internal/auth/context/loggercontext"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/ratelimit"
	"google.golang.org/genai"
)

type DemoService struct {
	GenAIClient   *genai.Client
	RateLimiter   *ratelimit.RateLimiter
	BookmarkModel *models.BookmarkModel
}

type DemoExtractRequest struct {
	URL string `json:"url"`
}

type DemoExtractResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

type DemoErrorResponse struct {
	Error        string `json:"error"`
	ShowSignup   bool   `json:"show_signup"`
	AttemptCount int    `json:"attempt_count"`
}

func (d *DemoService) ExtractContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	// Check rate limit first
	if d.RateLimiter == nil {
		logger.Errorw("rate limite is nil")
		http.Error(w, "rate limit is not working", http.StatusInternalServerError)
		return
	}
	ip := ratelimit.GetClientIP(r)
	if !d.RateLimiter.Allow(ip) {
		attemptCount := d.RateLimiter.GetAttemptCount(ip)
		showSignup := attemptCount >= 3

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)

		errorResp := DemoErrorResponse{
			Error:        "Rate limit exceeded. Please try again later.",
			ShowSignup:   showSignup,
			AttemptCount: attemptCount,
		}
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	var req DemoExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Validate URL
	if _, err := url.Parse(req.URL); err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	logger.Infow("request for demo", "ip", ip, "url", req.URL)

	bookmark, content, err := d.BookmarkModel.ExtractContentOnly(ctx, req.URL)
	if err != nil {
		http.Error(w, "Failed to extract content: "+err.Error(), http.StatusInternalServerError)
		return
	}

	summary := ""
	if d.GenAIClient != nil && len(content) > 100 {
		if aiData, err := d.BookmarkModel.PromptToGetAIData(content[:2000]); err == nil {
			summary = aiData.Summary
		}
	}

	response := DemoExtractResponse{
		Title:   bookmark.Title,
		Content: content,
		Summary: summary,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
