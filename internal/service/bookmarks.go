package service

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
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
		New    web.Template
		Edit   web.Template
		Index  web.Template
		Show   web.Template
		Search web.Template
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
	data.UserId = context.User(r.Context()).ID
	data.Link = r.FormValue("link")
	slog.Debug("creating bookmark", "link", data.Link, "userId", data.UserId)

	if !validations.IsURLValid(data.Link) {
		slog.Error("Invalid URL", "url", data.Link)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	bookmark, err := b.BookmarkModel.Create(data.Link, data.UserId, models.WebSource)
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

	var data struct {
		Link      string
		Title     string
		Id        types.BookmarkId
		Excerpt   string
		CreatedAt time.Time
		Thumbnail string
		Host      string
	}
	u, err := url.Parse(bookmark.Link)
	if err != nil {
		logger.Error("parsing url for bookmark edit", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	logger.Info("editing bookmark", "url", u)
	data.Link = bookmark.Link
	data.Host = u.Host
	data.Title = bookmark.Title
	data.Id = bookmark.BookmarkId
	data.Excerpt = bookmark.Excerpt
	if len(data.Excerpt) > 200 {
		data.Excerpt = data.Excerpt[:200] + "..."
	}
	data.CreatedAt = bookmark.CreatedAt
	data.Thumbnail = bookmark.ImageUrl
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
	http.Redirect(w, r, "/bookmarks", http.StatusFound)
}

func (b Bookmarks) Search(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}
	user := context.User(r.Context())

	results, err := b.BookmarkModel.Search(user.ID, query)
	if err != nil {
		logger.Error("searching bookmarks", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	var data struct {
		Bookmarks []types.BookmarkSearchResult
	}
	for _, r := range results {
		data.Bookmarks = append(data.Bookmarks, types.BookmarkSearchResult{
			Id:        r.BookmarkId,
			Title:     r.Title,
			Link:      r.Link,
			Headline:  r.Headline,
			Thumbnail: r.ImageUrl,
		})
	}
	if len(data.Bookmarks) == 0 {
		w.Write([]byte(`<p class="text-gray-500">Not found</p>`))
		return
	}

	logger.Info("searched bookmarks",
		"query", query,
		"count", len(data.Bookmarks))
	b.Templates.Search.Execute(w, r, data)
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
