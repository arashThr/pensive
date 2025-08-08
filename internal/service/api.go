package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arashthr/go-course/internal/auth/context/loggercontext"
	"github.com/arashthr/go-course/internal/auth/context/usercontext"
	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/types"
	"github.com/arashthr/go-course/internal/validations"
	"github.com/go-chi/chi/v5"
)

type Api struct {
	BookmarkModel *models.BookmarkModel
}

type ErrorResponse struct {
	Code    string `json:"errorCode"`
	Message string `json:"errorMessage"`
}

type Bookmark struct {
	Id      types.BookmarkId
	Title   string
	Link    string
	Excerpt string
}

// CheckBookmarkByLinkAPI checks if a bookmark exists by URL without creating it
//
// @Accept json
// @Produce json
// @Param url query string true "URL to check"
// @Success 200 {object} struct{exists bool, bookmark Bookmark} "Bookmark exists"
// @Failure 404 {object} ErrorResponse "Bookmark not found"
// @Failure 400 {object} ErrorResponse "Invalid URL"
// @Router /v1/api/bookmarks/check [get]
func (a *Api) CheckBookmarkByLinkAPI(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	link := r.URL.Query().Get("url")

	if link == "" {
		err := writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "URL parameter is required",
		})
		if err != nil {
			logger.Errorw("write response", "error", err)
		}
		return
	}

	if !validations.IsURLValid(link) {
		logger.Errorw("[api] invalid URL", "link", link)
		err := writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_URL",
			Message: fmt.Sprintf("Invalid URL: %v", link),
		})
		if err != nil {
			logger.Errorw("write response", "error", err)
		}
		return
	}

	bookmark, err := a.BookmarkModel.GetByLink(user.ID, link)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			var data struct {
				Exists bool `json:"exists"`
			}
			data.Exists = false
			err := writeResponse(w, data)
			if err != nil {
				logger.Errorw("write response", "error", err)
			}
			return
		}
		logger.Errorw("[api] failed to check bookmark", "error", err, "link", link)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "api: Something went wrong",
		})
		return
	}

	var data struct {
		Exists   bool     `json:"exists"`
		Bookmark Bookmark `json:"bookmark"`
	}
	data.Exists = true
	data.Bookmark = mapModelToBookmark(bookmark)
	err = writeResponse(w, data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *Api) IndexAPI(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	page := validations.GetPageOffset(r.FormValue("page"))
	bookmarks, _, err := a.BookmarkModel.GetByUserId(user.ID, page)
	if err != nil {
		logger.Errorw("fetching bookmarks", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data struct {
		Bookmarks []Bookmark
	}
	data.Bookmarks = make([]Bookmark, 0)
	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, mapModelToBookmark(&b))
	}
	err = writeResponse(w, data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

// CreateAPI handles the creation of a new bookmark.
//
// @Accept json
// @Produce json
// @Param data body struct{Link string; HtmlContent string; TextContent string; Title string; Excerpt string; Lang string; SiteName string; PublishedTime string; ImageUrl string} true "Bookmark link and content"
// @Success 200 {object} Bookmark
// @Failure 400 {object} ErrorResponse "Invalid request body or invalid URL"
// @Failure 500 {object} ErrorResponse "Failed to create bookmark"
// @Router /v1/api/bookmarks [post]
func (a *Api) CreateAPI(w http.ResponseWriter, r *http.Request) {
	var data types.CreateBookmarkRequest
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		logger.Errorw("[api] decoding request body", "error", err)
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}
	user := usercontext.User(ctx)

	if !validations.IsURLValid(data.Link) {
		logger.Errorw("[api] invalid URL", "link", data.Link)
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_URL",
			Message: fmt.Sprintf("Invalid URL: %v", data.Link),
		})
		return
	}

	if data.PublishedTime == nil {
		publishedTime := time.Now()
		data.PublishedTime = &publishedTime
	}

	logger.Infow("[api] creating bookmark", "link", data.Link, "userId", user.ID, "hasHtmlContent", data.HtmlContent != "", "hasTitle", data.Title != "")
	bookmark, err := a.BookmarkModel.CreateWithContent(ctx, data.Link, user, models.Api, &data)
	if err != nil {
		// Handle rate limit errors specifically
		if errors.Is(err, errors.ErrUnverifiedUserLimitExceeded) {
			logger.Infow("unverified user hit bookmark limit", "user_id", user.ID)
			writeErrorResponse(w, http.StatusForbidden, ErrorResponse{
				Code:    "UNVERIFIED_USER_LIMIT_EXCEEDED",
				Message: "Unverified account limit reached (10 bookmarks). Please verify your email to unlock unlimited bookmarks.",
			})
			return
		} else if errors.Is(err, errors.ErrDailyLimitExceeded) {
			logger.Infow("user hit daily limit", "user_id", user.ID)
			writeErrorResponse(w, http.StatusTooManyRequests, ErrorResponse{
				Code:    "DAILY_LIMIT_EXCEEDED",
				Message: "Daily bookmark limit exceeded. Upgrade to premium for 100 bookmarks/day.",
			})
			return
		}
		logger.Errorw("[api] failed to create bookmark", "error", err)

		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "CREATE_BOOKMARK",
			Message: fmt.Sprintf("Failed to create bookmark: %v", err),
		})
		return
	}
	logger.Infow("[api] created bookmark", "bookmarkId", bookmark.Id)
	err = writeResponse(w, mapModelToBookmark(bookmark))
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *Api) GetAPI(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark := a.getBookmark(w, r, userMustOwnBookmark)
	if bookmark == nil {
		writeErrorResponse(w, http.StatusNotFound, ErrorResponse{
			Code:    "NOT_FOUND",
			Message: fmt.Sprintf("Bookmark not found: %s", chi.URLParam(r, "id")),
		})
		return
	}
	err := writeResponse(w, mapModelToBookmark(bookmark))
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *Api) UpdateAPI(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark := a.getBookmark(w, r, userMustOwnBookmark)
	if bookmark == nil {
		return
	}
	var b Bookmark
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}
	bookmark.Title = b.Title
	err := a.BookmarkModel.Update(bookmark)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "UPDATE_BOOKMARK",
			Message: fmt.Sprintf("Failed to update bookmark: %v", err),
		})
		return
	}
	err = writeResponse(w, mapModelToBookmark(bookmark))
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *Api) DeleteByLinkAPI(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark := a.getBookmarkByLink(w, r)
	if bookmark == nil {
		return
	}

	err := a.BookmarkModel.Delete(bookmark.Id)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "DELETE_BOOKMARK",
			Message: fmt.Sprintf("Failed to delete bookmark: %v", err),
		})
		return
	}
	err = writeResponse(w, mapModelToBookmark(bookmark))
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
	logger.Infow("[api] deleted bookmark", "bookmarkId", bookmark.Id)
}

func (a *Api) DeleteAPI(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	bookmark := a.getBookmark(w, r, userMustOwnBookmark)
	if bookmark == nil {
		return
	}
	err := a.BookmarkModel.Delete(bookmark.Id)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "DELETE_BOOKMARK",
			Message: fmt.Sprintf("Failed to delete bookmark: %v", err),
		})
		return
	}
	var data struct {
		Id types.BookmarkId `json:"id"`
	}
	data.Id = bookmark.Id
	err = writeResponse(w, &data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

// SearchAPI handles the search for bookmarks based on a query.
// @Produce json
// @Param query query string true "Search query"
// @Success 200 {object} bookmarkSearchResult `json:"bookmarks"`
// @Failure 400 {object} ErrorResponse "Query is required"
// @Failure 500 {object} ErrorResponse "Something went wrong"
// @Router /v1/api/bookmarks/search [get]
func (a *Api) SearchAPI(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}
	user := usercontext.User(r.Context())

	results, err := a.BookmarkModel.Search(user, query)
	if err != nil {
		logger.Errorw("searching bookmarks", "error", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	var data struct {
		Bookmarks []types.BookmarkSearchResult
	}
	for _, r := range results {
		data.Bookmarks = append(data.Bookmarks, types.BookmarkSearchResult{
			Id:        r.Id,
			Title:     r.Title,
			Link:      r.Link,
			Hostname:  validations.ExtractHostname(r.Link),
			Headline:  r.Headline,
			Thumbnail: r.ImageUrl,
		})
	}
	err = writeResponse(w, data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *Api) getBookmark(w http.ResponseWriter, r *http.Request, opts ...bookmarkOpts) *models.Bookmark {
	id := chi.URLParam(r, "id")
	logger := loggercontext.Logger(r.Context())
	if strings.TrimSpace(id) == "" {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: "Bookmark ID is required",
		})
		return nil
	}
	bookmark, err := a.BookmarkModel.GetById(types.BookmarkId(id))
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeErrorResponse(w, http.StatusNotFound, ErrorResponse{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Bookmark not found: %s", id),
			})
			return nil
		}
		logger.Errorw("[api] get bookmark by ID", "error", err, "id", id)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "api: Something went wrong",
		})
		return nil
	}

	for _, opt := range opts {
		if err := opt(w, r, bookmark); err != nil {
			logger.Errorw("running opts on getBookmark", "error", err)
			writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "INTERNAL_ERROR",
				Message: "api: Something went wrong",
			})
			return nil
		}
	}
	return bookmark
}

func (a *Api) getBookmarkByLink(w http.ResponseWriter, r *http.Request) *models.Bookmark {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	var data struct {
		Link string
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		logger.Errorw("[api] decoding request body", "error", err)
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return nil
	}

	if !validations.IsURLValid(data.Link) {
		logger.Errorw("[api] invalid URL", "link", data.Link)
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_URL",
			Message: fmt.Sprintf("Invalid URL: %v", data.Link),
		})
		return nil
	}
	bookmark, err := a.BookmarkModel.GetByLink(user.ID, data.Link)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeErrorResponse(w, http.StatusNotFound, ErrorResponse{
				Code:    "NOT_FOUND",
				Message: fmt.Sprintf("Bookmark not found: %s", data.Link),
			})
			return nil
		}
		logger.Errorw("[api] failed to create bookmark", "error", err, "link", data.Link)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "api: Something went wrong",
		})
		return nil
	}
	return bookmark
}

func writeResponse(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return err
	}
	return nil
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, errResp ErrorResponse) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
		return err
	}
	return nil
}

func mapModelToBookmark(b *models.Bookmark) Bookmark {
	return Bookmark{
		Id:      b.Id,
		Title:   b.Title,
		Link:    b.Link,
		Excerpt: b.Excerpt,
	}
}
