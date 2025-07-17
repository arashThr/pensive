package service

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
	"github.com/arashthr/go-course/web"
	"github.com/go-chi/chi/v5"
)

type Bookmarks struct {
	Templates struct {
		New                  web.Template
		Edit                 web.Template
		Index                web.Template
		Show                 web.Template
		Markdown             web.Template
		MarkdownNotAvailable web.Template
	}
	BookmarkModel *models.BookmarkModel
}

func (b Bookmarks) New(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Link string
	}
	data.Link = r.FormValue("link")
	b.Templates.New.Execute(w, r, data)
}

func (b Bookmarks) Create(w http.ResponseWriter, r *http.Request) {
	var data struct {
		UserId types.UserId
		Link   string
	}
	user := context.User(r.Context())
	data.UserId = user.ID
	data.Link = r.FormValue("link")
	slog.Debug("creating bookmark", "link", data.Link, "userId", data.UserId)

	if !validations.IsURLValid(data.Link) {
		slog.Error("Invalid URL", "url", data.Link)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	bookmark, err := b.BookmarkModel.Create(data.Link, data.UserId, models.WebSource, user.SubscriptionStatus)
	if err != nil {
		b.Templates.New.Execute(w, r, data, web.NavbarMessage{
			Message: err.Error(),
			IsError: true,
		})
		return
	}

	// TODO: Load the same page with the message: Bookmark added
	editPath := fmt.Sprintf("/bookmarks/%s/edit", bookmark.BookmarkId)
	http.Redirect(w, r, editPath, http.StatusFound)
}

func (b Bookmarks) Edit(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	user := context.User(r.Context())

	var data struct {
		Link      string
		Title     string
		Id        types.BookmarkId
		Excerpt   string
		CreatedAt time.Time
		Thumbnail string
		Host      string
		// AI-generated fields for premium users
		AISummary *string
		AIExcerpt *string
		AITags    *string
		IsPremium bool
	}
	host := validations.ExtractHostname(bookmark.Link)
	logger.Info("editing bookmark", "url", host)
	data.Link = bookmark.Link
	data.Host = host
	data.Title = bookmark.Title
	data.Id = bookmark.BookmarkId
	data.Excerpt = bookmark.Excerpt
	if len(data.Excerpt) > 200 {
		data.Excerpt = data.Excerpt[:200] + "..."
	}
	data.CreatedAt = bookmark.CreatedAt
	data.Thumbnail = bookmark.ImageUrl
	data.IsPremium = user.SubscriptionStatus == models.SubscriptionStatusPremium

	logger.Info("Is premiun", "prem", data.IsPremium)

	// For premium users, fetch AI-generated content
	if data.IsPremium {
		data.AISummary = bookmark.AISummary
		data.AIExcerpt = bookmark.AIExcerpt
		data.AITags = bookmark.AITags
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
		Id:        bookmark.BookmarkId,
		Excerpt:   bookmark.Excerpt,
		CreatedAt: bookmark.CreatedAt,
	}
	b.Templates.Edit.Execute(w, r, data, web.NavbarMessage{
		Message: "Bookmark updated",
		IsError: false,
	})
}

func (b Bookmarks) Index(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	page := validations.GetPageOffset(r.FormValue("page"))
	bookmarks, morePages, err := b.BookmarkModel.GetByUserId(user.ID, page)
	if err != nil {
		log.Printf("bookmark by user id: %v", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type Bookmark struct {
		Id        types.BookmarkId
		Title     string
		Link      string
		CreatedAt string
	}
	type PagesData struct {
		Previous int
		Current  int
		Next     int
	}
	var data struct {
		Pages     PagesData
		MorePages bool
		Bookmarks []Bookmark
	}
	data.Pages = PagesData{
		Previous: page - 1,
		Current:  page,
		Next:     page + 1,
	}
	data.MorePages = morePages
	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, Bookmark{
			Id:        b.BookmarkId,
			Title:     b.Title,
			Link:      b.Link,
			CreatedAt: b.CreatedAt.Format("Jan 02"),
		})
	}

	b.Templates.Index.Execute(w, r, data)
}

func (b Bookmarks) Delete(w http.ResponseWriter, r *http.Request) {
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}
	err = b.BookmarkModel.Delete(bookmark.BookmarkId)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/home", http.StatusFound)
}

// GetFullBookmark handles GET /v1/bookmarks/{id}/full and returns the full content of a bookmark.
func (b Bookmarks) GetFullBookmark(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}
	fullContent, err := b.BookmarkModel.GetBookmarkContent(bookmark.BookmarkId)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			http.Error(w, fmt.Sprintf("Bookmark content not found for ID: %s", bookmark.BookmarkId), http.StatusNotFound)
			return
		}
		logger.Error("[bookmarks] get bookmark content by ID", "error", err, "id", bookmark.BookmarkId)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type Response struct {
		Id          types.BookmarkId `json:"id"`
		Title       string           `json:"title"`
		Link        string           `json:"link"`
		Excerpt     string           `json:"excerpt"`
		CreatedAt   time.Time        `json:"created_at,omitempty"`
		SiteName    string           `json:"site_name,omitempty"`
		Source      string           `json:"source,omitempty"`
		ImageUrl    string           `json:"image_url,omitempty"`
		ArticleLang string           `json:"article_lang,omitempty"`
		Content     string           `json:"content"`
	}
	resp := Response{
		Id:          bookmark.BookmarkId,
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
		logger.Error("encoding response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetBookmarkMarkdown handles GET /bookmarks/{id}/markdown and returns the AI-generated markdown content
func (b Bookmarks) GetBookmarkMarkdown(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	markdownContent, err := b.BookmarkModel.GetBookmarkMarkdown(bookmark.BookmarkId)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			// If no markdown content is found, show a user-friendly message
			var data struct {
				Id types.BookmarkId
			}
			data.Id = bookmark.BookmarkId
			b.Templates.MarkdownNotAvailable.Execute(w, r, data)
			return
		}
		logger.Error("[bookmarks] get bookmark markdown by ID", "error", err, "id", bookmark.BookmarkId)
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
	data.Id = bookmark.BookmarkId
	data.Title = bookmark.Title
	data.Link = bookmark.Link
	data.MarkdownContent = markdownContent

	b.Templates.Markdown.Execute(w, r, data)
}

func (b Bookmarks) getBookmark(w http.ResponseWriter, r *http.Request, opts ...bookmarkOpts) (*models.Bookmark, error) {
	id := chi.URLParam(r, "id")
	bookmark, err := b.BookmarkModel.GetById(types.BookmarkId(id))
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			http.Error(w, "Bookmark not found", http.StatusNotFound)
			return nil, err
		}
		log.Print(err)
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
	user := context.User(r.Context())
	if user.ID != bookmark.UserId {
		http.Error(w, "User does not have access to the bookmark", http.StatusForbidden)
		return fmt.Errorf("user does not have access to the bookmark")
	}
	return nil
}
