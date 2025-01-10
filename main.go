package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func bookmarkHandler(w http.ResponseWriter, r *http.Request) {
	bookmarkId := chi.URLParam(r, "bookmark_id")
	fmt.Fprint(w, "Hello "+bookmarkId)
}

func setupDb() *pgxpool.Pool {
	cfg := models.DefaultPostgresConfig()
	err := models.Migrate(cfg.PgConnectionString())
	if err != nil {
		panic(err)
	}

	pool, err := models.Open(cfg)
	if err != nil {
		log.Fatalf("connecting to db: %v", err)
	}
	return pool
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Database
	pool := setupDb()
	defer pool.Close()

	// Services
	userService := models.UserService{
		Pool: pool,
	}
	sessionService := models.SessionService{
		Pool: pool,
	}

	// Middlewares
	umw := controllers.UserMiddleware{
		SessionService: &sessionService,
	}
	csrfKey := os.Getenv("CSRF_TOKEN")
	csrfMw := csrf.Protect([]byte(csrfKey), csrf.Secure(false))

	// Controllers
	usersController := controllers.Users{
		UserService:    &userService,
		SessionService: &sessionService,
	}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))

	// Routes
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(csrfMw)
	r.Use(umw.SetUser)

	r.Get("/", controllers.StaticHandler(
		views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml")),
	))
	r.Get("/contact", controllers.StaticHandler(
		views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml")),
	))
	r.Get("/faq", controllers.FAQ(
		views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml")),
	))
	r.Get("/signup", usersController.New)
	r.Get("/signin", usersController.SignIn)
	r.Post("/signin", usersController.ProcessSignIn)
	r.Post("/signout", usersController.ProcessSignOut)
	r.Post("/users", usersController.Create)
	r.Route("/users/me", func(r chi.Router) {
		r.Use(umw.RequireUser)
		r.Get("/", usersController.CurrentUser)
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	fmt.Println("Starting server on port 8000")
	http.ListenAndServe("localhost:8000", r)
}
