package controllers

import (
	"net/http"

	"github.com/arashthr/go-course/models"
)

type Bookmarks struct {
	Templates struct {
		New Template
	}
	BookmarkService *models.BookmarkService
}

func (g Bookmarks) New(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Link string
	}
	data.Link = r.FormValue("link")
	g.Templates.New.Execute(w, r, data)
}
