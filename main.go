package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
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
	applyMigrations(cfg.PgConnectionString())

	pool, err := models.Open(cfg)
	if err != nil {
		log.Fatalf("connecting to db: %v", err)
	}
	return pool
}

//go:embed db/migrations/*.sql
var fs embed.FS

func applyMigrations(connString string) {
	driver, err := iofs.New(fs, "db/migrations")
	if err != nil {
		log.Fatal(err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", driver, connString)
	if err != nil {
		log.Fatalf("creating migration instance: %v", err)
	}
	err = m.Up()
	if err != nil {
		if err == migrate.ErrNoChange {
			log.Println("no change")
			return
		}
		log.Fatalf("applying migrations: %v", err)
	} else {
		log.Println("migrations applied")
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	pool := setupDb()
	defer pool.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	csrfKey := os.Getenv("CSRF_TOKEN")
	r.Use(csrf.Protect([]byte(csrfKey), csrf.Secure(false)))

	tpl := views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml"))
	r.Get("/", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml"))
	r.Get("/contact", controllers.StaticHandler(tpl))

	tpl = views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml"))
	r.Get("/faq", controllers.FAQ(tpl))

	userService := models.UserService{
		Pool: pool,
	}
	sessionService := models.SessionService{
		Pool: pool,
	}
	usersController := controllers.Users{
		UserService:    &userService,
		SessionService: &sessionService,
	}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))
	r.Get("/signup", usersController.New)
	r.Get("/signin", usersController.SignIn)
	r.Post("/signin", usersController.ProcessSignIn)
	r.Post("/users", usersController.Create)
	r.Get("/users/me", usersController.CurrentUser)

	r.Route("/bookmarks", func(r chi.Router) {
		r.Get("/{bookmark_id}", bookmarkHandler)
	})
	fmt.Println("Starting server on port 8000")
	http.ListenAndServe("localhost:8000", r)
}
