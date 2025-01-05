package main

import (
	"fmt"
	"net/http"

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

	tpl := views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml"))
	r.Get("/", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml"))
	r.Get("/contact", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml"))
	r.Get("/faq", controllers.FAQ(tpl))

	usersController := controllers.Users{}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	r.Get("/signup", usersController.New)
	r.Post("/users", usersController.Create)

	r.Route("/bookmarks", func(r chi.Router) {
		r.Get("/{bookmark_id}", bookmarkHandler)
	})
	fmt.Println("Starting server on port 8000")
	// http.ListenAndServe("localhost:8000", http.HandlerFunc(pathHandler))
	http.ListenAndServe("localhost:8000", r)
}
