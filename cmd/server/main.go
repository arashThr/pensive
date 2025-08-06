package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/arashthr/go-course/internal/auth"
	authcontext "github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/db"
	"github.com/arashthr/go-course/internal/logging"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/internal/service/importer"
	"github.com/arashthr/go-course/web"
	"github.com/arashthr/go-course/web/views"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/genai"
)

func setupDb(cfg config.PostgresConfig) (*pgxpool.Pool, error) {
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

	logging.Init(cfg)
	defer logging.Sync()

	err = run(cfg)
	if err != nil {
		panic(err)
	}
}

func run(cfg *config.AppConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	pool, err := setupDb(cfg.PSQL)
	if err != nil {
		return err
	}
	defer pool.Close()

	genAIClient, err := genai.NewClient(ctx, nil)
	if err != nil {
		logging.Logger.Errorw("failed to create Gemini client", "error", err)
	}

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
		Pool:        pool,
		GenAIClient: genAIClient,
	}
	telegramModel := &models.TelegramService{
		Pool: pool,
	}
	importJobModel := &models.ImportJobModel{
		Pool: pool,
	}
	authTokenService := models.NewAuthTokenService(pool)

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
		Domain:               cfg.Domain,
		TurnstileConfig:      cfg.Turnstile,
		UserService:          userService,
		SessionService:       sessionService,
		PasswordResetService: passwordResetService,
		AuthTokenService:     authTokenService,
		EmailService:         emailService,
		TokenModel:           tokenModel,
	}
	usersController.AuthConfig.AllowPasswordAuth = cfg.Auth.AllowPasswordAuth
	usersController.AuthConfig.AllowPasswordlessAuth = cfg.Auth.AllowPasswordlessAuth

	usersController.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))
	usersController.Templates.ForgotPassword = views.Must(views.ParseTemplate("forgot-pw.gohtml", "tailwind.gohtml"))
	usersController.Templates.CheckYourEmail = views.Must(views.ParseTemplate("check-your-email.gohtml", "tailwind.gohtml"))
	usersController.Templates.ResetPassword = views.Must(views.ParseTemplate("reset-password.gohtml", "tailwind.gohtml"))
	usersController.Templates.UserPage = views.Must(views.ParseTemplate("user/user-page.gohtml", "tailwind.gohtml"))
	usersController.Templates.Token = views.Must(views.ParseTemplate("user/token.gohtml"))
	usersController.Templates.ProfileTab = views.Must(views.ParseTemplate("user/profile-tab.gohtml"))
	usersController.Templates.TokensTab = views.Must(views.ParseTemplate("user/tokens-tab.gohtml"))
	usersController.Templates.ImportExportTab = views.Must(views.ParseTemplate("user/import-export-tab.gohtml"))
	usersController.Templates.Subscribe = views.Must(views.ParseTemplate("user/subscribe.gohtml", "tailwind.gohtml"))
	usersController.Templates.PasswordlessNew = views.Must(views.ParseTemplate("passwordless-signup.gohtml", "tailwind.gohtml"))
	usersController.Templates.PasswordlessSignIn = views.Must(views.ParseTemplate("passwordless-signin.gohtml", "tailwind.gohtml"))
	usersController.Templates.PasswordlessCheckEmail = views.Must(views.ParseTemplate("passwordless-check-email.gohtml", "tailwind.gohtml"))

	bookmarksController := service.Bookmarks{
		BookmarkModel: bookmarksModel,
	}
	bookmarksController.Templates.New = views.Must(views.ParseTemplate("bookmarks/new.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Edit = views.Must(views.ParseTemplate("bookmarks/edit.gohtml", "tailwind.gohtml", "bookmarks/markdown.gohtml"))
	bookmarksController.Templates.Index = views.Must(views.ParseTemplate("bookmarks/index.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.Markdown = views.Must(views.ParseTemplate("bookmarks/markdown.gohtml", "tailwind.gohtml"))
	bookmarksController.Templates.MarkdownNotAvailable = views.Must(views.ParseTemplate("bookmarks/markdown-not-available.gohtml", "tailwind.gohtml"))

	homeController := service.Home{
		BookmarkModel: bookmarksModel,
	}
	homeController.Templates.Home = views.Must(views.ParseTemplate("home/home.gohtml", "tailwind.gohtml", "home/recent-results.gohtml"))
	homeController.Templates.SearchResults = views.Must(views.ParseTemplate("home/search-results.gohtml"))
	homeController.Templates.RecentResults = views.Must(views.ParseTemplate("home/recent-results.gohtml"))

	importerController := service.Importer{
		ImportJobModel: importJobModel,
		BookmarkModel:  bookmarksModel,
	}
	importerController.Templates.PocketImport = views.Must(views.ParseTemplate("user/pocket-import.gohtml", "tailwind.gohtml"))
	importerController.Templates.ImportProcessing = views.Must(views.ParseTemplate("user/import-processing.gohtml", "tailwind.gohtml"))
	importerController.Templates.ImportStatus = views.Must(views.ParseTemplate("user/import-status.gohtml", "tailwind.gohtml"))

	apiController := service.Api{
		BookmarkModel: bookmarksModel,
	}

	tokenController := service.Token{
		TokenModel: tokenModel,
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

	// OAuth controllers
	githubController := auth.NewGitHubOAuth(cfg.GitHub, cfg.Domain, userService, sessionService)
	googleController := auth.NewGoogleOAuth(cfg.Google, cfg.Domain, userService, sessionService)

	// Start import processor in background
	importProcessor := importer.ImportProcessor{
		ImportJobModel: importJobModel,
		BookmarkModel:  bookmarksModel,
		UserModel:      userService,
	}
	go importProcessor.Start(ctx)

	// Middlewares
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Use(amw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production"))

		r.Get("/ping", healthCheck)
		r.Post("/stripe-webhooks", stripController.Webhook)

		r.Route("/v1", func(r chi.Router) {
			r.Use(amw.RequireUser)
			// TODO: Remove this
			r.Get("/ping", tokenController.AuthenticatedPing)
			r.Route("/tokens", func(r chi.Router) {
				r.Delete("/current", tokenController.DeleteToken)
			})
			r.Get("/user", apiController.CurrentUserAPI)
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", apiController.IndexAPI)
				r.Post("/", apiController.CreateAPI)
				r.Delete("/", apiController.DeleteByLinkAPI)
				r.Get("/check", apiController.CheckBookmarkByLinkAPI)
				r.Get("/{id}", apiController.GetAPI)
				r.Put("/{id}", apiController.UpdateAPI)
				r.Delete("/{id}", apiController.DeleteAPI)
				r.Get("/search", apiController.SearchAPI)
			})
		})
	})

	// Web routes
	r.Group(func(r chi.Router) {
		r.Use(csrfMw)
		r.Use(umw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production"))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			user := authcontext.User(r.Context())
			if user != nil {
				// User is authenticated, redirect to /home
				http.Redirect(w, r, "/home", http.StatusFound)
				return
			}
			// User is not authenticated, show the regular home page
			web.StaticHandler(
				"Home",
				views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml")),
			)(w, r)
		})
		r.Get("/contact", web.StaticHandler(
			"Contact",
			views.Must(views.ParseTemplate("contact.gohtml", "tailwind.gohtml")),
		))
		r.Get("/faq", web.FAQ(
			views.Must(views.ParseTemplate("faq.gohtml", "tailwind.gohtml")),
		))
		r.Get("/privacy", web.StaticHandler(
			"Privacy",
			views.Must(views.ParseTemplate("privacy.gohtml", "tailwind.gohtml")),
		))
		r.Get("/integrations", web.StaticHandler(
			"Extensions",
			views.Must(views.ParseTemplate("integrations.gohtml", "tailwind.gohtml")),
		))
		r.Get("/pocket", web.StaticHandler(
			"Pocket import",
			views.Must(views.ParseTemplate("pocket-intro.gohtml", "tailwind.gohtml")),
		))
		r.Get("/signup", usersController.New)
		r.Get("/signin", usersController.SignIn)
		r.Post("/signin", usersController.ProcessSignIn)
		r.Post("/signout", usersController.ProcessSignOut)
		r.Get("/forgot-pw", usersController.ForgotPassword)
		r.Post("/forgot-pw", usersController.ProcessForgotPassword)
		r.Get("/reset-password", usersController.ResetPassword)
		r.Post("/reset-password", usersController.ProcessResetPassword)

		// Passwordless authentication routes
		r.Route("/auth/passwordless", func(r chi.Router) {
			r.Get("/signup", usersController.PasswordlessNew)
			r.Post("/signup", usersController.ProcessPasswordlessSignup)
			r.Get("/signin", usersController.PasswordlessSignIn)
			r.Post("/signin", usersController.ProcessPasswordlessSignIn)
			r.Get("/verify", usersController.VerifyPasswordlessAuth)
		})
		r.Route("/home", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/", homeController.Index)
			r.Get("/search", homeController.Search)
		})
		r.Route("/users", func(r chi.Router) {
			r.Post("/", usersController.Create)
			// Subscriptions
			r.Get("/subscribe", usersController.Subscribe)
			// Auth
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Get("/me", usersController.CurrentUser)
				r.Get("/tab-content", usersController.TabContent)
				r.Post("/delete-token", usersController.DeleteToken)
			})
			// Import/export
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Get("/pocket-import", importerController.PocketImport)
				r.Post("/pocket-import", importerController.ProcessImport)
				r.Post("/export", importerController.ProcessExport)
				r.Get("/import-status", importerController.ImportStatus)
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
				r.Get("/{id}/full", bookmarksController.GetFullBookmark)
				r.Get("/{id}/markdown", bookmarksController.GetBookmarkMarkdown)
				r.Get("/{id}/markdown-content", bookmarksController.GetBookmarkMarkdownHTMX)
			})
		})

		r.Route("/oauth", func(r chi.Router) {
			r.Get("/github", githubController.RedirectToGitHub)
			r.Get("/github/callback", githubController.HandleCallback)

			r.Get("/google", googleController.RedirectToGoogle)
			r.Get("/google/callback", googleController.HandleCallback)
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

func LoggerMiddleware(isProduction bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t1 := time.Now()
			ctx := r.Context()
			reqLogger := logging.Logger.With(
				"req_path", r.URL.Path,
				"req_method", r.Method,
			)

			if user := authcontext.User(ctx); user != nil {
				reqLogger = reqLogger.With("user", user.ID)
			}
			ctx = authcontext.WithLogger(ctx, reqLogger)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				reqLogger.Debugw("http request", "from", r.RemoteAddr, "status", ww.Status(), "size", ww.BytesWritten(), "duration", time.Since(t1))
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
