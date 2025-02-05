package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type config struct {
	PSQL models.PostgresConfig
	SMTP models.SMTPConfig
	CSRF struct {
		Key    string
		Secure bool
	}
	Server struct {
		Address string
	}
}

func loadEnvConfig() (*config, error) {
	var cfg config
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("loading .env file: %w", err)
	}
	// DB
	cfg.PSQL = models.DefaultPostgresConfig()

	// SMTP
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return nil, err
	}
	cfg.SMTP = models.SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     port,
		Username: os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASS"),
	}

	// CSRF
	cfg.CSRF.Key = os.Getenv("CSRF_TOKEN")
	cfg.CSRF.Secure = os.Getenv("CSRF_SECURE") == "true"

	// Server
	cfg.Server.Address = os.Getenv("SERVER_ADDRESS")

	return &cfg, nil
}

func setupDb(cfg models.PostgresConfig) (*pgxpool.Pool, error) {
	err := models.Migrate(cfg.PgConnectionString())
	if err != nil {
		panic(err)
	}

	pool, err := models.Open(cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %v", err)
	}
	return pool, nil
}

func main() {
	cfg, err := loadEnvConfig()
	if err != nil {
		panic(err)
	}
	err = run(cfg)
	if err != nil {
		panic(err)
	}
}

func run(cfg *config) error {
	// Database
	pool, err := setupDb(cfg.PSQL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Services
	userService := &models.UserService{
		Pool: pool,
	}
	sessionService := &models.SessionService{
		Pool: pool,
	}
	passwordResetService := &models.PasswordResetService{
		Pool: pool,
		Now:  func() time.Time { return time.Now() },
	}
	emailService := models.NewEmailService(cfg.SMTP)
	apiService := &models.ApiService{
		Pool: pool,
	}

	// Middlewares
	umw := controllers.UserMiddleware{
		SessionService: sessionService,
	}
	csrfMw := csrf.Protect([]byte(cfg.CSRF.Key), csrf.Secure(cfg.CSRF.Secure))

	// Controllers
	usersController := controllers.Users{
		UserService:          userService,
		SessionService:       sessionService,
		PasswordResetService: passwordResetService,
		EmailService:         emailService,
		ApiService:           apiService,
	}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))
	usersController.Templates.ForgotPassword = views.Must(views.ParseTemplate("forgot-pw.gohtml", "tailwind.gohtml"))
	usersController.Templates.CheckYourEmail = views.Must(views.ParseTemplate("check-your-email.gohtml", "tailwind.gohtml"))
	usersController.Templates.ResetPassword = views.Must(views.ParseTemplate("reset-password.gohtml", "tailwind.gohtml"))
	usersController.Templates.UserPage = views.Must(views.ParseTemplate("user-page.gohtml", "tailwind.gohtml"))

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
	r.Post("/users", usersController.Create)
	r.Get("/signin", usersController.SignIn)
	r.Post("/signin", usersController.ProcessSignIn)
	r.Post("/signout", usersController.ProcessSignOut)
	r.Get("/forgot-pw", usersController.ForgotPassword)
	r.Post("/forgot-pw", usersController.ProcessForgotPassword)
	r.Get("/reset-password", usersController.ResetPassword)
	r.Post("/reset-password", usersController.ProcessResetPassword)
	r.Route("/users", func(r chi.Router) {
		r.Use(umw.RequireUser)
		r.Get("/me", usersController.CurrentUser)
		r.Get("/generate-token", usersController.GenerateToken)
	})

	r.Route("/v1/api", func(r chi.Router) {
		// TODO: Put the API token middleware here
		// r.Use(umw.RequireUser)
	})

	assetHandler := http.FileServer(http.Dir("assets"))
	r.Get("/assets/*", http.StripPrefix("/assets", assetHandler).ServeHTTP)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	fmt.Printf("Starting server on %s...", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, r)
}
