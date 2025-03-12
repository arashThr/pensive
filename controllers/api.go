package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/controllers/validations"
	"github.com/arashthr/go-course/errors"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/types"
	"github.com/go-chi/chi/v5"
)

type Api struct {
	BookmarkService *models.BookmarkService
}

type ErrorResponse struct {
	Code    string `json:"errorCode"`
	Message string `json:"errorMessage"`
}

type Bookmark struct {
	Id    types.BookmarkId
	Title string
	Link  string
}

func (a *Api) IndexAPI(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	bookmarks, err := a.BookmarkService.ByUserId(user.ID)
	if err != nil {
		log.Printf("fetching bookmarks: %v", err)
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
	writeResponse(w, data)
}

func (a *Api) CreateAPI(w http.ResponseWriter, r *http.Request) {
	var data struct {
		UserId types.UserId
		Link   string
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}
	data.UserId = context.User(r.Context()).ID

	if !validations.IsURLValid(data.Link) {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "INVALID_URL",
			Message: fmt.Sprintf("Invalid URL: %v", data.Link),
		})
		return
	}

	bookmark, err := a.BookmarkService.Create(data.Link, data.UserId, models.Api)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "CREATE_BOOKMARK",
			Message: fmt.Sprintf("Failed to create bookmark: %v", err),
		})
		return
	}
	writeResponse(w, mapModelToBookmark(bookmark))
}

func (a *Api) GetAPI(w http.ResponseWriter, r *http.Request) {
	bookmark := a.getBookmark(w, r, userMustOwnBookmark)
	if bookmark == nil {
		return
	}
	writeResponse(w, mapModelToBookmark(bookmark))
}

func (a *Api) UpdateAPI(w http.ResponseWriter, r *http.Request) {
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
	err := a.BookmarkService.Update(bookmark)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "UPDATE_BOOKMARK",
			Message: fmt.Sprintf("Failed to update bookmark: %v", err),
		})
		return
	}
	writeResponse(w, mapModelToBookmark(bookmark))
}

func (a *Api) DeleteAPI(w http.ResponseWriter, r *http.Request) {
	bookmark := a.getBookmark(w, r, userMustOwnBookmark)
	if bookmark == nil {
		return
	}
	err := a.BookmarkService.Delete(bookmark.BookmarkId)
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
	data.Id = bookmark.BookmarkId
	writeResponse(w, &data)
}

func (a *Api) getBookmark(w http.ResponseWriter, r *http.Request, opts ...bookmarkOpts) *models.Bookmark {
	id := chi.URLParam(r, "id")
	bookmark, err := a.BookmarkService.ById(types.BookmarkId(id))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeErrorResponse(w, http.StatusNotFound, ErrorResponse{
				Code:    "NOT_FOUND",
				Message: "Bookmark not found",
			})
			return nil
		}
		log.Print(err)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "INTERNAL_ERROR",
			Message: "api: Something went wrong",
		})
		return nil
	}

	for _, opt := range opts {
		if err := opt(w, r, bookmark); err != nil {
			log.Printf("Error in running opts on getBookmark: %v", err)
			writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
				Code:    "INTERNAL_ERROR",
				Message: "api: Something went wrong",
			})
			return nil
		}
	}
	return bookmark
}

func writeResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("encoding response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, errResp ErrorResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		log.Printf("encoding error response: %v", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

func mapModelToBookmark(b *models.Bookmark) Bookmark {
	return Bookmark{
		Id:    b.BookmarkId,
		Title: b.Title,
		Link:  b.Link,
	}
}
