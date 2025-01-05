package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func bookmarkHandler(w http.ResponseWriter, r *http.Request) {
	bookmarkId := chi.URLParam(r, "bookmark_id")
	fmt.Fprint(w, "Hello "+bookmarkId)
}

func setupDb() *pgxpool.Pool {
	cfg := models.DefaultPostgresConfig()
	pool, err := models.Open(cfg)
	if err != nil {
		log.Fatalf("connecting to db: %v", err)
	}
	defer pool.Close()
	return pool
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	pool := setupDb()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	tpl := views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml"))
	r.Get("/", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml"))
	r.Get("/contact", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml"))
	r.Get("/faq", controllers.FAQ(tpl))

	userService := models.UserService{
		Pool: pool,
	}
	usersController := controllers.Users{
		UserService: &userService,
	}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	r.Get("/signup", usersController.New)
	r.Post("/users", usersController.Create)

	r.Route("/bookmarks", func(r chi.Router) {
		r.Get("/{bookmark_id}", bookmarkHandler)
	})
	fmt.Println("Starting server on port 8000")
	http.ListenAndServe("localhost:8000", r)
}
