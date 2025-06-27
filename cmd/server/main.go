package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/arashthr/go-course/internal/auth"
	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/db"
	"github.com/arashthr/go-course/internal/logging"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/web"
	"github.com/arashthr/go-course/web/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupDb(cfg db.PostgresConfig) (*pgxpool.Pool, error) {
	err := db.Migrate(cfg.PgConnectionString())
	if err != nil {
		panic(err)
	}

	pool, err := db.Open(cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %v", err)
	}
	return pool, nil
}

func main() {
	cfg, err := config.LoadEnvConfig()
	if err != nil {
		panic(err)
	}
	log := logging.GetLogger(cfg.Environment == "production")
	slog.SetDefault(log)
	err = run(cfg)
	if err != nil {
		panic(err)
	}
}

func run(cfg *config.AppConfig) error {
	// Database
	pool, err := setupDb(cfg.PSQL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Services
	userService := &models.UserModel{
		Pool: pool,
	}
	sessionService := &models.SessionService{
		Pool: pool,
	}
	passwordResetService := &models.PasswordResetService{
		Pool: pool,
		Now:  func() time.Time { return time.Now() },
	}
	emailService := service.NewEmailService(cfg.SMTP)
	tokenModel := &models.TokenModel{
		Pool: pool,
	}
	stripeModel := &models.StripeModel{
		Pool: pool,
	}
	bookmarksModel := &models.BookmarkModel{
		Pool: pool,
	}
	telegramModel := &models.TelegramService{
		Pool: pool,
	}

	// Middlewares
	umw := auth.UserMiddleware{
		SessionService: sessionService,
	}
	amw := auth.ApiMiddleware{
		TokenModel: tokenModel,
	}
	csrfMw := csrf.Protect(
		[]byte(cfg.CSRF.Key),
		csrf.Secure(cfg.CSRF.Secure),
		csrf.Path("/"),
	)

	// Controllers
	usersController := auth.Users{
		UserService:          userService,
		SessionService:       sessionService,
		PasswordResetService: passwordResetService,
		EmailService:         emailService,
		TokenModel:           tokenModel,
	}
	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))
	usersController.Templates.ForgotPassword = views.Must(views.ParseTemplate("forgot-pw.gohtml", "tailwind.gohtml"))
	usersController.Templates.CheckYourEmail = views.Must(views.ParseTemplate("check-your-email.gohtml", "tailwind.gohtml"))
	usersController.Templates.ResetPassword = views.Must(views.ParseTemplate("reset-password.gohtml", "tailwind.gohtml"))
	usersController.Templates.UserPage = views.Must(views.ParseTemplate("user/user-page.gohtml", "tailwind.gohtml"))
	usersController.Templates.Token = views.Must(views.ParseTemplate("user/token.gohtml"))

	bookmarksController := service.Bookmarks{
		BookmarkModel: bookmarksModel,
	}
	bookmarksController.Templates.New = views.Must(views.ParseTemplate("bookmarks/new.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Edit = views.Must(views.ParseTemplate("bookmarks/edit.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Index = views.Must(views.ParseTemplate("bookmarks/index.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Search = views.Must(views.ParseTemplate("bookmarks/search.gohtml", "tailwind.gohtml"))

	apiController := service.Api{
		BookmarkModel: bookmarksModel,
	}

	stripController := service.Stripe{
		Domain:              cfg.Domain,
		PriceId:             cfg.Stripe.PriceId,
		StripeWebhookSecret: cfg.Stripe.StripeWebhookSecret,
		StripeModel:         stripeModel,
	}
	stripController.Templates.Success = views.Must(views.ParseTemplate("payments/success.gohtml", "tailwind.gohtml"))
	stripController.Templates.Cancel = views.Must(views.ParseTemplate("payments/cancel.gohtml", "tailwind.gohtml"))

	extensionController := auth.Extension{
		TokenModel: tokenModel,
	}

	telegramController := auth.Telegram{
		TelegramModel: telegramModel,
		BotName:       cfg.Telegram.BotName,
	}

	// Middlewares
	r := chi.NewRouter()
	// TODO: Use slog for logging requests
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(logging.LoggerMiddleware(cfg.Environment == "production"))

	// Routes
	r.Route("/api", func(r chi.Router) {
		r.Use(amw.SetUser)

		r.Get("/ping", healthCheck)
		r.Post("/stripe-webhooks", stripController.Webhook)

		r.Route("/v1", func(r chi.Router) {
			r.Use(amw.RequireUser)
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", apiController.IndexAPI)
				r.Post("/", apiController.CreateAPI)
				r.Delete("/", apiController.DeleteByLinkAPI)
				r.Get("/{id}", apiController.GetAPI)
				r.Put("/{id}", apiController.UpdateAPI)
				r.Delete("/{id}", apiController.DeleteAPI)
				r.Get("/search", apiController.SearchAPI)
			})
		})
	})

	getHomePage := func(w http.ResponseWriter, r *http.Request) {
		user := context.User(r.Context())
		home := "home.gohtml"
		if user != nil {
			home = "user/home.gohtml"
		}
		web.StaticHandler(
			views.Must(views.ParseTemplate(home, "tailwind.gohtml")),
		)(w, r)
	}

	r.Group(func(r chi.Router) {
		r.Use(csrfMw)
		r.Use(umw.SetUser)

		r.Get("/", getHomePage)
		r.Get("/contact", web.StaticHandler(
			views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml")),
		))
		r.Get("/faq", web.FAQ(
			views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml")),
		))
		r.Get("/integrations", web.StaticHandler(
			views.Must(views.ParseTemplate("integrations.gohtml", "tailwind.gohtml")),
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
			r.Get("/subscribe", web.StaticHandler(
				views.Must(views.ParseTemplate("user/subscribe.gohtml", "tailwind.gohtml")),
			))
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Get("/me", usersController.CurrentUser)
				r.Post("/generate-token", usersController.GenerateToken)
				r.Post("/delete-token", usersController.DeleteToken)
			})
		})
		r.Route("/payments", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Post("/create-checkout-session", stripController.CreateCheckoutSession)
			r.Post("/create-portal-session", stripController.CreatePortalSession)
			r.Get("/portal-session", stripController.GoToBillingPortal)
			r.Get("/success", stripController.Success)
			r.Get("/cancel", stripController.Cancel)
		})

		r.Route("/extension", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", extensionController.GenerateToken)
		})

		r.Route("/telegram", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", telegramController.RedirectWithAuthToken)
		})

		assetHandler := http.FileServer(http.Dir("./web/assets"))
		r.Get("/assets/*", http.StripPrefix("/assets", assetHandler).ServeHTTP)
		r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("favicon")
			http.ServeFile(w, r, "./web/assets/favicon.ico")
		})

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
				r.Get("/search", bookmarksController.Search)
			})
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	fmt.Printf("Starting server on %s...", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, r)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}
