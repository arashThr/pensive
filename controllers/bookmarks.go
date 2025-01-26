package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/models"
	"github.com/go-chi/chi/v5"
	"github.com/go-shiori/go-readability"
)

type Bookmarks struct {
	Templates struct {
		New   Template
		Edit  Template
		Index Template
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
		UserId uint
		Link   string
	}
	data.UserId = context.User(r.Context()).ID
	data.Link = r.FormValue("link")

	if !isURLValid(data.Link) {
		log.Printf("Invalid URL: %v", data.Link)
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	// TODO: Check if the link already exists
	article, err := readability.FromURL(data.Link, 5*time.Second)
	if err != nil {
		log.Printf("readability from url: %v", err)
		http.Error(w, "Could not read the article", http.StatusInternalServerError)
		return
	}

	bookmark, err := b.BookmarkService.Create(data.Link, article.Title, data.UserId)
	if err != nil {
		b.Templates.New.Execute(w, r, data, NavbarMessage{
			Message: err.Error(),
			IsError: true,
		})
		return
	}

	// TODO: Load the same page with the message: Bookmark added
	editPath := fmt.Sprintf("/bookmarks/%d/edit", bookmark.ID)
	http.Redirect(w, r, editPath, http.StatusFound)
}

func (b Bookmarks) Edit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid bookmark id", http.StatusBadRequest)
		return
	}
	bookmark, err := b.BookmarkService.ById(uint(id))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "Bookmark not found", http.StatusNotFound)
			return
		}
		log.Print(err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	user := context.User(r.Context())
	if user.ID != bookmark.UserId {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	var data struct {
		Link  string
		Title string
		Id    uint
	}
	data.Link = bookmark.Link
	data.Title = bookmark.Title
	data.Id = bookmark.ID
	b.Templates.Edit.Execute(w, r, data)
}

func (b Bookmarks) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid bookmark id", http.StatusBadRequest)
		return
	}
	bookmark, err := b.BookmarkService.ById(uint(id))
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "Bookmark not found", http.StatusNotFound)
			return
		}
		log.Print(err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	user := context.User(r.Context())
	if user.ID != bookmark.UserId {
		http.Error(w, "Unauthorized", http.StatusForbidden)
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
		Id    uint
	}
	data.Link = bookmark.Link
	data.Title = bookmark.Title
	data.Id = bookmark.ID
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
		Id    uint
		Title string
		Link  string
	}
	var data struct {
		Bookmarks []Bookmark
	}
	for _, b := range bookmarks {
		data.Bookmarks = append(data.Bookmarks, Bookmark{
			Id:    b.ID,
			Title: b.Title,
			Link:  b.Link,
		})
	}

	b.Templates.Index.Execute(w, r, data)
}

func isURLValid(link string) bool {
	if link == "" || len(link) > 2048 {
		return false
	}
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	return u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}
