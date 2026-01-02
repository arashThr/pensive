package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/types"
	"github.com/arashthr/pensive/internal/validations"
	"github.com/arashthr/pensive/web"
	"github.com/go-chi/chi/v5"
)

type Bookmarks struct {
	Templates struct {
		New                  web.Template
		Edit                 web.Template
		Show                 web.Template
		Markdown             web.Template
		MarkdownNotAvailable web.Template
	}
	BookmarkModel *models.BookmarkRepo
}

func (b Bookmarks) New(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
		Link  string
	}
	data.Title = "New Bookmark"
	data.Link = r.FormValue("link")
	b.Templates.New.Execute(w, r, data)
}

func (b Bookmarks) Create(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title  string
		UserId types.UserId
		Link   string
	}
	ctx := r.Context()
	user := usercontext.User(ctx)
	data.Title = "New Bookmark"
	data.UserId = user.ID
	data.Link = r.FormValue("link")
	logging.Logger.Debugw("creating bookmark", "link", data.Link, "userId", data.UserId)

	if !validations.IsURLValid(data.Link) {
		logging.Logger.Errorw("Invalid URL", "url", data.Link)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	bookmark, err := b.BookmarkModel.Create(ctx, data.Link, user, models.WebSource)
	if err != nil {
		var message string
		if errors.Is(err, errors.ErrUnverifiedUserLimitExceeded) {
			message = "Unverified account limit reached (10 bookmarks). Please verify your email to unlock unlimited bookmarks."
		} else if errors.Is(err, errors.ErrDailyLimitExceeded) {
			message = "Daily bookmark limit exceeded. Upgrade to premium for 100 bookmarks/day."
		} else {
			message = err.Error()
		}

		b.Templates.New.Execute(w, r, data, web.NavbarMessage{
			Message: message,
			IsError: true,
		})
		return
	}

	// TODO: Load the same page with the message: Bookmark added
	editPath := fmt.Sprintf("/bookmarks/%s", bookmark.Id)
	http.Redirect(w, r, editPath, http.StatusFound)
}

func (b Bookmarks) Edit(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	user := usercontext.User(r.Context())

	var data struct {
		Link      string
		Title     string
		Id        types.BookmarkId
		Excerpt   string
		CreatedAt time.Time
		Thumbnail string
		Host      string
		// AI-generated fields for premium users
		AISummary string
		AIExcerpt string
		AITags    string
		IsPremium bool
	}
	host := validations.ExtractHostname(bookmark.Link)
	logger.Infow("editing bookmark", "url", host)
	data.Link = bookmark.Link
	data.Host = host
	data.Title = bookmark.Title
	data.Id = bookmark.Id
	data.Excerpt = bookmark.Excerpt
	if len(data.Excerpt) > 200 {
		data.Excerpt = data.Excerpt[:200] + "..."
	}
	data.CreatedAt = bookmark.CreatedAt
	data.Thumbnail = bookmark.ImageUrl
	data.IsPremium = user.IsSubscriptionPremium()

	logger.Infow("Subscription status", "status", user.SubscriptionStatus, "is_premium", data.IsPremium, "user", user.ID)

	// AI-generated content is now available for all users
	if bookmark.AISummary != nil {
		data.AISummary = *bookmark.AISummary
	}
	if bookmark.AIExcerpt != nil {
		data.AIExcerpt = *bookmark.AIExcerpt
	}
	if bookmark.AITags != nil {
		data.AITags = *bookmark.AITags
	}

	b.Templates.Edit.Execute(w, r, data)
}

func (b Bookmarks) Update(w http.ResponseWriter, r *http.Request) {
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	bookmark.Title = r.FormValue("title")
	err = b.BookmarkModel.Update(bookmark)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	data := struct {
		Link      string
		Title     string
		Id        types.BookmarkId
		Excerpt   string
		CreatedAt time.Time
	}{
		Link:      bookmark.Link,
		Title:     bookmark.Title,
		Id:        bookmark.Id,
		Excerpt:   bookmark.Excerpt,
		CreatedAt: bookmark.CreatedAt,
	}
	b.Templates.Edit.Execute(w, r, data, web.NavbarMessage{
		Message: "Bookmark updated",
		IsError: false,
	})
}

func (b Bookmarks) Delete(w http.ResponseWriter, r *http.Request) {
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}
	err = b.BookmarkModel.Delete(bookmark.Id)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/home", http.StatusFound)
}

// GetFullBookmark handles GET /v1/bookmarks/{id}/full and returns the full content of a bookmark.
func (b Bookmarks) GetFullBookmark(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}
	fullContent, err := b.BookmarkModel.GetBookmarkContent(bookmark.Id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			http.Error(w, fmt.Sprintf("Bookmark content not found for ID: %s", bookmark.Id), http.StatusNotFound)
			return
		}
		logger.Errorw("[bookmarks] get bookmark content by ID", "error", err, "id", bookmark.Id)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type Response struct {
		Id          types.BookmarkId `json:"id"`
		Title       string           `json:"title"`
		Link        string           `json:"link"`
		Excerpt     string           `json:"excerpt"`
		CreatedAt   time.Time        `json:"created_at"`
		SiteName    string           `json:"site_name,omitempty"`
		Source      string           `json:"source,omitempty"`
		ImageUrl    string           `json:"image_url,omitempty"`
		ArticleLang string           `json:"article_lang,omitempty"`
		Content     string           `json:"content"`
	}
	resp := Response{
		Id:          bookmark.Id,
		Title:       bookmark.Title,
		Link:        bookmark.Link,
		Excerpt:     bookmark.Excerpt,
		CreatedAt:   bookmark.CreatedAt,
		SiteName:    bookmark.SiteName,
		Source:      bookmark.Source,
		ImageUrl:    bookmark.ImageUrl,
		ArticleLang: bookmark.ArticleLang,
		Content:     fullContent,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Errorw("encoding response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetBookmarkMarkdown handles GET /bookmarks/{id}/markdown and returns the AI-generated markdown content
func (b Bookmarks) GetBookmarkMarkdown(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	markdownContent, err := b.BookmarkModel.GetBookmarkMarkdown(bookmark.Id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			// If no markdown content is found, show a user-friendly message
			var data struct {
				Title string
				Id    types.BookmarkId
			}
			data.Title = "Markdown Not Available"
			data.Id = bookmark.Id
			b.Templates.MarkdownNotAvailable.Execute(w, r, data)
			return
		}
		logger.Errorw("[bookmarks] get bookmark markdown by ID", "error", err, "id", bookmark.Id)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// Render the markdown content using template
	var data struct {
		Id              types.BookmarkId
		Title           string
		Link            string
		MarkdownContent string
	}
	data.Id = bookmark.Id
	data.Title = bookmark.Title
	data.Link = bookmark.Link
	data.MarkdownContent = markdownContent

	b.Templates.Markdown.Execute(w, r, data)
}

// GetBookmarkMarkdownHTMX handles HTMX requests for GET /bookmarks/{id}/markdown-content and returns just the markdown content
func (b Bookmarks) GetBookmarkMarkdownHTMX(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	markdownContent, err := b.BookmarkModel.GetBookmarkMarkdown(bookmark.Id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<div class="p-4 text-center text-gray-500">
				<p>No markdown content available for this bookmark.</p>
			</div>`))
			return
		}
		logger.Errorw("[bookmarks] get bookmark markdown by ID", "error", err, "id", bookmark.Id)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// Return raw markdown content for client-side rendering
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(markdownContent))
}

// ReportBookmark handles POST /bookmarks/{id}/report and sends a report about content capture issues
func (b Bookmarks) ReportBookmark(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	user := usercontext.User(r.Context())

	// Send report via Telegram
	message := fmt.Sprintf("🚨 Content Capture Issue Report\n\nURL: %s\nTitle: %s\nUser: %s\nBookmark ID: %s",
		bookmark.Link, bookmark.Title, user.Email, bookmark.Id)

	err = logging.Telegram.SendMessage(message)
	if err != nil {
		logger.Errorw("failed to send report via telegram", "error", err, "bookmark_id", bookmark.Id)
		http.Error(w, "Failed to send report", http.StatusInternalServerError)
		return
	}

	logger.Infow("bookmark reported", "bookmark_id", bookmark.Id, "user_id", user.ID)
	w.WriteHeader(http.StatusOK)
}

func (b Bookmarks) getBookmark(w http.ResponseWriter, r *http.Request, opts ...bookmarkOpts) (*models.Bookmark, error) {
	logger := loggercontext.Logger(r.Context())
	id := chi.URLParam(r, "id")
	bookmark, err := b.BookmarkModel.GetById(types.BookmarkId(id))
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			http.Error(w, "Bookmark not found", http.StatusNotFound)
			return nil, err
		}
		logger.Errorw("get bookmark", "error", err, "bookmark_id", id)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return nil, err
	}

	for _, opt := range opts {
		if err := opt(w, r, bookmark); err != nil {
			return nil, err
		}
	}

	return bookmark, nil
}

type bookmarkOpts func(http.ResponseWriter, *http.Request, *models.Bookmark) error

func userMustOwnBookmark(w http.ResponseWriter, r *http.Request, bookmark *models.Bookmark) error {
	user := usercontext.User(r.Context())
	if user.ID != bookmark.UserId {
		http.Error(w, "User does not have access to the bookmark", http.StatusForbidden)
		return fmt.Errorf("user does not have access to the bookmark")
	}
	return nil
}
