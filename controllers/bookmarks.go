package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/controllers/validations"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/types"
	"github.com/go-chi/chi/v5"
)

type Bookmarks struct {
	Templates struct {
		New    Template
		Edit   Template
		Index  Template
		Show   Template
		Search Template
	}
	BookmarkService *models.BookmarkService
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

	if !validations.IsURLValid(data.Link) {
		log.Printf("Invalid URL: %v", data.Link)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	bookmark, err := b.BookmarkService.Create(data.Link, data.UserId)
	if err != nil {
		b.Templates.New.Execute(w, r, data, NavbarMessage{
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
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	var data struct {
		Link  string
		Title string
		Id    types.BookmarkId
	}
	data.Link = bookmark.Link
	data.Title = bookmark.Title
	data.Id = bookmark.BookmarkId
	b.Templates.Edit.Execute(w, r, data)
}

func (b Bookmarks) Update(w http.ResponseWriter, r *http.Request) {
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}

	bookmark.Title = r.FormValue("title")
	err = b.BookmarkService.Update(bookmark)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	var data struct {
		Link  string
		Title string
		Id    types.BookmarkId
	}
	data.Link = bookmark.Link
	data.Title = bookmark.Title
	data.Id = bookmark.BookmarkId
	b.Templates.Edit.Execute(w, r, data, NavbarMessage{
		Message: "Bookmark updated",
		IsError: false,
	})
}

func (b Bookmarks) Index(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	bookmarks, err := b.BookmarkService.ByUserId(user.ID)
	if err != nil {
		log.Printf("bookmark by user id: %v", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type Bookmark struct {
		Id    types.BookmarkId
		Title string
		Link  string
	}
	var data struct {
		Bookmarks []Bookmark
	}
	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, Bookmark{
			Id:    b.BookmarkId,
			Title: b.Title,
			Link:  b.Link,
		})
	}

	b.Templates.Index.Execute(w, r, data)
}

func (b Bookmarks) Delete(w http.ResponseWriter, r *http.Request) {
	bookmark, err := b.getBookmark(w, r, userMustOwnBookmark)
	if err != nil {
		return
	}
	err = b.BookmarkService.Delete(bookmark.BookmarkId)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/bookmarks", http.StatusFound)
}

func (b Bookmarks) Search(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "Query is required", http.StatusBadRequest)
		return
	}
	user := context.User(r.Context())

	bookmarks, err := b.BookmarkService.Search(user.ID, query)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	type Bookmark struct {
		Id    types.BookmarkId
		Title string
		Link  string
	}
	var data struct {
		Bookmarks []Bookmark
	}
	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, Bookmark{
			Id:    b.BookmarkId,
			Title: b.Title,
			Link:  b.Link,
		})
	}
	logger := context.Logger(r.Context())
	logger.Info("searched bookmarks",
		"query", query,
		"count", len(data.Bookmarks))
	b.Templates.Search.Execute(w, r, data)
}

func (b Bookmarks) getBookmark(w http.ResponseWriter, r *http.Request, opts ...bookmarkOpts) (*models.Bookmark, error) {
	id := chi.URLParam(r, "id")
	bookmark, err := b.BookmarkService.ById(types.BookmarkId(id))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
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
