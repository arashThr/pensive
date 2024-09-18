package main

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func bookmarkHandler(w http.ResponseWriter, r *http.Request) {
	bookmarkId := chi.URLParam(r, "bookmark_id")
	fmt.Fprint(w, "Hello "+bookmarkId)
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	tpl := views.Must(views.ParseTemplate(filepath.Join("home.gohtml")))
	r.Get("/", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate(filepath.Join("contact.gohtml")))
	r.Get("/contact", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate(filepath.Join("faq.gohtml")))
	r.Get("/faq", controllers.StaticHandler(tpl))

	r.Route("/bookmarks", func(r chi.Router) {
		r.Get("/{bookmark_id}", bookmarkHandler)
	})
	fmt.Println("Starting server on port 8000")
	// http.ListenAndServe("localhost:8000", http.HandlerFunc(pathHandler))
	http.ListenAndServe("localhost:8000", r)
}
