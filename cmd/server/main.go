package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arashthr/go-course/controllers"
	"github.com/arashthr/go-course/logger"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v81"
)

type StripeConfig struct {
	Key                 string
	PriceId             string
	StripeWebhookSecret string
}

type config struct {
	Domain string
	PSQL   models.PostgresConfig
	SMTP   models.SMTPConfig
	CSRF   struct {
		Key    string
		Secure bool
	}
	Server struct {
		Address string
	}
	Stripe StripeConfig
}

func loadEnvConfig() (*config, error) {
	var cfg config
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("loading .env file: %w", err)
	}

	cfg.Domain = os.Getenv("DOMAIN")

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

	// Stripe
	cfg.Stripe = StripeConfig{
		Key:                 os.Getenv("STRIPE_KEY"),
		PriceId:             os.Getenv("STRIPE_PRICE_ID"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
	// Or set stripe.Key
	stripe.Key = os.Getenv("STRIPE_KEY")

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
	log := logger.GetLogger()
	slog.SetDefault(log)
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
	stripeService := &models.StripeService{
		Pool: pool,
	}
	bookmarksService := &models.BookmarkService{
		Pool: pool,
	}

	// Middlewares
	umw := controllers.UserMiddleware{
		SessionService: sessionService,
	}
	amw := controllers.ApiMiddleware{
		ApiService: apiService,
	}
	csrfMw := csrf.Protect(
		[]byte(cfg.CSRF.Key),
		csrf.Secure(cfg.CSRF.Secure),
		csrf.Path("/"),
	)

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

	bookmarksController := controllers.Bookmarks{
		BookmarkService: bookmarksService,
	}
	bookmarksController.Templates.New = views.Must(views.ParseTemplate("bookmarks/new.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Edit = views.Must(views.ParseTemplate("bookmarks/edit.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Index = views.Must(views.ParseTemplate("bookmarks/index.gohtml", "tailwind.gohtml"))

	apiController := controllers.Api{
		BookmarkService: bookmarksService,
	}

	stripController := controllers.Stripe{
		Domain:              cfg.Domain,
		PriceId:             cfg.Stripe.PriceId,
		StripeWebhookSecret: cfg.Stripe.StripeWebhookSecret,
		StripeService:       stripeService,
	}
	stripController.Templates.Success = views.Must(views.ParseTemplate("payments/success.gohtml", "tailwind.gohtml"))
	stripController.Templates.Cancel = views.Must(views.ParseTemplate("payments/cancel.gohtml", "tailwind.gohtml"))

	// Routes
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				amw.SetUser(next).ServeHTTP(w, r)
			} else {
				csrfMw(umw.SetUser(next)).ServeHTTP(w, r)
			}
		})
	})

	r.Route("/api", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})
		r.Post("/stripe-webhooks", stripController.Webhook)
		r.Route("/v1", func(r chi.Router) {
			r.Use(amw.RequireUser)
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", apiController.IndexAPI)
				r.Post("/", apiController.CreateAPI)
				r.Get("/{id}", apiController.GetAPI)
				r.Put("/{id}", apiController.UpdateAPI)
				r.Delete("/{id}", apiController.DeleteAPI)
			})
		})
	})

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
	r.Get("/forgot-pw", usersController.ForgotPassword)
	r.Post("/forgot-pw", usersController.ProcessForgotPassword)
	r.Get("/reset-password", usersController.ResetPassword)
	r.Post("/reset-password", usersController.ProcessResetPassword)
	r.Route("/users", func(r chi.Router) {
		r.Post("/", usersController.Create)
		r.Group(func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/me", usersController.CurrentUser)
			r.Get("/generate-token", usersController.GenerateToken)
		})
	})
	r.Route("/payments", func(r chi.Router) {
		r.Use(umw.RequireUser)
		r.Post("/create-checkout-session", stripController.CreateCheckoutSession)
		r.Post("/create-portal-session", stripController.CreatePortalSession)
		r.Get("/success", stripController.Success)
		r.Get("/cancel", stripController.Cancel)
	})

	assetHandler := http.FileServer(http.Dir("assets"))
	r.Get("/assets/*", http.StripPrefix("/assets", assetHandler).ServeHTTP)
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("favicon")
		http.ServeFile(w, r, "./assets/favicon.ico")
	})

	r.Get("/bookmarks/new", bookmarksController.New)
	r.Route("/bookmarks", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			// For routes that are accessible by user
			r.Use(umw.RequireUser)
			r.Get("/", bookmarksController.Index)
			r.Post("/", bookmarksController.Create)
			r.Get("/new", bookmarksController.New)
			r.Get("/{id}/edit", bookmarksController.Edit)
			r.Post("/{id}", bookmarksController.Update)
			r.Post("/{id}/delete", bookmarksController.Delete)
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	fmt.Printf("Starting server on %s...", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, r)
}
