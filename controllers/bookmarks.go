package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/models"
	"github.com/go-chi/chi/v5"
)

type Bookmarks struct {
	Templates struct {
		New  Template
		Edit Template
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

	bookmark, err := b.BookmarkService.Create(data.Link, data.UserId)
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
