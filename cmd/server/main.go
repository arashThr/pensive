/*
Pensive - Your searchable memory of the web
Copyright (C) 2025  Arash Taher

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/arashthr/pensive/internal/auth"
	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/config"
	"github.com/arashthr/pensive/internal/db"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/service"
	"github.com/arashthr/pensive/internal/service/importer"
	"github.com/arashthr/pensive/web"
	"github.com/arashthr/pensive/web/views"
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

	// Database
	pool, err := setupDb(cfg.PSQL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	err = run(cfg, pool)
	if err != nil {
		panic(err)
	}
}

// ServiceContainer holds all the repositories and services
type ServiceContainer struct {
	// Repositories
	UserRepo          *models.UserRepo
	SessionRepo       *models.SessionRepo
	PasswordResetRepo *models.PasswordResetRepo
	TokenRepo         *models.TokenRepo
	StripeRepo        *models.StripeModel
	BookmarkRepo      *models.BookmarkRepo
	TelegramRepo      *models.TelegramRepo
	ImportJobRepo     *models.ImportJobRepo
	AuthTokenRepo     *models.AuthTokenService

	// Services
	EmailService     *service.EmailService
	UsersService     auth.Users
	BookmarksService service.Bookmarks
	HomeService      service.Home
	ImporterService  service.Importer
	UserService      service.User
	ApiService       service.Api
	TokenService     service.Token
	StripeService    service.Stripe
	ExtensionService auth.Extension
	TelegramService  auth.Telegram

	// Import processor
	ImportProcessor importer.ImportProcessor
}

// newServiceContainer creates and initializes all repositories and services
func newServiceContainer(cfg *config.AppConfig, pool *pgxpool.Pool, ctx context.Context) (*ServiceContainer, error) {
	genAIClient, err := genai.NewClient(ctx, nil)
	if err != nil {
		logging.Logger.Errorw("failed to create Gemini client", "error", err)
	}

	// Repositories
	userRepo := &models.UserRepo{
		Pool: pool,
	}
	sessionRepo := &models.SessionRepo{
		Pool: pool,
	}
	passwordResetRepo := &models.PasswordResetRepo{
		Pool: pool,
		Now:  func() time.Time { return time.Now() },
	}
	tokenRepo := &models.TokenRepo{
		Pool: pool,
	}
	stripeRepo := models.NewStripeRepo(cfg.Stripe.Key, pool)
	bookmarkRepo := &models.BookmarkRepo{
		Pool:        pool,
		GenAIClient: genAIClient,
	}
	telegramRepo := &models.TelegramRepo{
		Pool: pool,
	}
	importJobRepo := &models.ImportJobRepo{
		Pool: pool,
	}
	authTokenRepo := models.NewAuthTokenRepo(pool)

	// Services
	emailService := service.NewEmailService(cfg.SMTP)

	usersService := auth.Users{
		Domain:               cfg.Domain,
		TurnstileConfig:      cfg.Turnstile,
		UserService:          userRepo,
		SessionService:       sessionRepo,
		PasswordResetService: passwordResetRepo,
		AuthTokenService:     authTokenRepo,
		EmailService:         emailService,
		TokenModel:           tokenRepo,
	}

	// Initialize user service templates
	usersService.Templates.New = views.Must(views.ParseTemplate("signup.gohtml", "tailwind.gohtml"))
	usersService.Templates.SignIn = views.Must(views.ParseTemplate("signin.gohtml", "tailwind.gohtml"))
	usersService.Templates.ForgotPassword = views.Must(views.ParseTemplate("forgot-pw.gohtml", "tailwind.gohtml"))
	usersService.Templates.CheckYourEmail = views.Must(views.ParseTemplate("check-your-email.gohtml", "tailwind.gohtml"))
	usersService.Templates.ResetPassword = views.Must(views.ParseTemplate("reset-password.gohtml", "tailwind.gohtml"))
	usersService.Templates.UserPage = views.Must(views.ParseTemplate("user/user-page.gohtml", "tailwind.gohtml"))
	usersService.Templates.Token = views.Must(views.ParseTemplate("user/token.gohtml"))
	usersService.Templates.ProfileTab = views.Must(views.ParseTemplate("user/profile-tab.gohtml"))
	usersService.Templates.TokensTab = views.Must(views.ParseTemplate("user/tokens-tab.gohtml"))
	usersService.Templates.ImportExportTab = views.Must(views.ParseTemplate("user/import-export-tab.gohtml"))
	usersService.Templates.DataManagementTab = views.Must(views.ParseTemplate("user/data-management-tab.gohtml"))
	usersService.Templates.Subscribe = views.Must(views.ParseTemplate("user/subscribe.gohtml", "tailwind.gohtml"))
	usersService.Templates.PasswordlessNew = views.Must(views.ParseTemplate("passwordless-signup.gohtml", "tailwind.gohtml"))
	usersService.Templates.PasswordlessSignIn = views.Must(views.ParseTemplate("passwordless-signin.gohtml", "tailwind.gohtml"))
	usersService.Templates.PasswordlessCheckEmail = views.Must(views.ParseTemplate("passwordless-check-email.gohtml", "tailwind.gohtml"))

	bookmarksService := service.Bookmarks{
		BookmarkModel: bookmarkRepo,
	}
	bookmarksService.Templates.New = views.Must(views.ParseTemplate("bookmarks/new.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.Edit = views.Must(views.ParseTemplate("bookmarks/edit.gohtml", "tailwind.gohtml", "bookmarks/markdown.gohtml"))
	bookmarksService.Templates.Index = views.Must(views.ParseTemplate("bookmarks/index.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.Markdown = views.Must(views.ParseTemplate("bookmarks/markdown.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.MarkdownNotAvailable = views.Must(views.ParseTemplate("bookmarks/markdown-not-available.gohtml", "tailwind.gohtml"))

	homeService := service.Home{
		BookmarkModel: bookmarkRepo,
	}
	homeService.Templates.Home = views.Must(views.ParseTemplate("home/home.gohtml", "tailwind.gohtml", "home/recent-results.gohtml"))
	homeService.Templates.SearchResults = views.Must(views.ParseTemplate("home/search-results.gohtml"))
	homeService.Templates.RecentResults = views.Must(views.ParseTemplate("home/recent-results.gohtml"))
	homeService.Templates.ChatAnswer = views.Must(views.ParseTemplate("home/chat-answer.gohtml"))

	importerService := service.Importer{
		ImportJobModel: importJobRepo,
		BookmarkModel:  bookmarkRepo,
	}
	importerService.Templates.PocketImport = views.Must(views.ParseTemplate("user/pocket-import.gohtml", "tailwind.gohtml"))
	importerService.Templates.ImportProcessing = views.Must(views.ParseTemplate("user/import-processing.gohtml", "tailwind.gohtml"))
	importerService.Templates.ImportStatus = views.Must(views.ParseTemplate("user/import-status.gohtml", "tailwind.gohtml"))

	userService := service.User{
		BookmarkModel:    bookmarkRepo,
		AuthTokenService: authTokenRepo,
		EmailService:     emailService,
		Domain:           cfg.Domain,
	}

	apiService := service.Api{
		BookmarkModel: bookmarkRepo,
	}

	tokenService := service.Token{
		TokenModel: tokenRepo,
	}

	stripeService := service.Stripe{
		Domain:              cfg.Domain,
		PriceId:             cfg.Stripe.PriceId,
		StripeWebhookSecret: cfg.Stripe.StripeWebhookSecret,
		StripeModel:         stripeRepo,
	}
	stripeService.Templates.Success = views.Must(views.ParseTemplate("payments/success.gohtml", "tailwind.gohtml"))
	stripeService.Templates.Cancel = views.Must(views.ParseTemplate("payments/cancel.gohtml", "tailwind.gohtml"))

	extensionService := auth.Extension{
		TokenModel: tokenRepo,
	}

	telegramService := auth.Telegram{
		TelegramModel: telegramRepo,
		BotName:       cfg.Telegram.BotName,
	}

	importProcessor := importer.ImportProcessor{
		ImportJobModel: importJobRepo,
		BookmarkModel:  bookmarkRepo,
		UserModel:      userRepo,
	}

	return &ServiceContainer{
		// Repositories
		UserRepo:          userRepo,
		SessionRepo:       sessionRepo,
		PasswordResetRepo: passwordResetRepo,
		TokenRepo:         tokenRepo,
		StripeRepo:        stripeRepo,
		BookmarkRepo:      bookmarkRepo,
		TelegramRepo:      telegramRepo,
		ImportJobRepo:     importJobRepo,
		AuthTokenRepo:     authTokenRepo,

		// Services
		EmailService:     emailService,
		UsersService:     usersService,
		BookmarksService: bookmarksService,
		HomeService:      homeService,
		ImporterService:  importerService,
		UserService:      userService,
		ApiService:       apiService,
		TokenService:     tokenService,
		StripeService:    stripeService,
		ExtensionService: extensionService,
		TelegramService:  telegramService,

		// Import processor
		ImportProcessor: importProcessor,
	}, nil
}

func run(cfg *config.AppConfig, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create service container with all repositories and services
	container, err := newServiceContainer(cfg, pool, ctx)
	if err != nil {
		return fmt.Errorf("failed to create service container: %w", err)
	}

	// Start import processor in background
	go container.ImportProcessor.Start(ctx)

	// Create routes with the service container
	r := Routes(cfg, container)

	fmt.Printf("Starting server on %s...", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, r)
}

func Routes(cfg *config.AppConfig, c *ServiceContainer) *chi.Mux {
	githubService := auth.NewGitHubOAuth(cfg.GitHub, cfg.Domain, c.UserRepo, c.SessionRepo)
	googleService := auth.NewGoogleOAuth(cfg.Google, cfg.Domain, c.UserRepo, c.SessionRepo)

	// Middlewares
	umw := auth.UserMiddleware{
		SessionService: c.SessionRepo,
	}
	amw := auth.ApiMiddleware{
		TokenModel: c.TokenRepo,
	}
	csrfMw := csrf.Protect(
		[]byte(cfg.CSRF.Key),
		csrf.Secure(cfg.CSRF.Secure),
		csrf.Path("/"),
	)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Use(amw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production", "api"))

		r.Get("/ping", healthCheck)
		r.Post("/stripe-webhooks", c.StripeService.Webhook)

		r.Route("/v1", func(r chi.Router) {
			r.Use(amw.RequireUser)
			// TODO: Remove this
			r.Get("/ping", c.TokenService.AuthenticatedPing)
			r.Route("/tokens", func(r chi.Router) {
				r.Delete("/current", c.TokenService.DeleteToken)
			})
			r.Route("/user", func(r chi.Router) {
				r.Get("/", c.UserService.CurrentUserAPI)
				r.Post("/request-verification", c.UserService.RequestVerificationEmailAPI)
			})
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", c.ApiService.IndexAPI)
				r.Post("/", c.ApiService.CreateAPI)
				r.Delete("/", c.ApiService.DeleteByLinkAPI)
				r.Get("/check", c.ApiService.CheckBookmarkByLinkAPI)
				r.Get("/{id}", c.ApiService.GetAPI)
				r.Put("/{id}", c.ApiService.UpdateAPI)
				r.Delete("/{id}", c.ApiService.DeleteAPI)
				r.Get("/search", c.ApiService.SearchAPI)
			})
		})
	})

	// Web routes
	r.Group(func(r chi.Router) {
		r.Use(umw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production", "web"))
		r.Use(csrfMw)

		r.Get("/", web.StaticHandler(
			"Home",
			views.Must(views.ParseTemplate("home.gohtml", "tailwind.gohtml")),
		))
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
		r.Get("/signup", c.UsersService.New)
		r.Get("/signin", c.UsersService.SignIn)
		r.Post("/signin", c.UsersService.ProcessSignIn)
		r.Post("/signout", c.UsersService.ProcessSignOut)
		r.Get("/forgot-pw", c.UsersService.ForgotPassword)
		r.Post("/forgot-pw", c.UsersService.ProcessForgotPassword)
		r.Get("/reset-password", c.UsersService.ResetPassword)
		r.Post("/reset-password", c.UsersService.ProcessResetPassword)

		// Email verification routes
		r.Route("/auth", func(r chi.Router) {
			r.Get("/verify-email", c.UsersService.VerifyEmail)

			r.Route("/resend-verification", func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Post("/", c.UsersService.ResendVerificationEmail)
			})

			r.Route("/passwordless", func(r chi.Router) {
				r.Get("/signup", c.UsersService.PasswordlessNew)
				r.Post("/signup", c.UsersService.ProcessPasswordlessSignup)
				r.Get("/signin", c.UsersService.PasswordlessSignIn)
				r.Post("/signin", c.UsersService.ProcessPasswordlessSignIn)
				r.Get("/verify", c.UsersService.VerifyPasswordlessAuth)
			})
		})
		r.Route("/home", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/", c.HomeService.Index)
			r.Get("/search", c.HomeService.Search)
			r.Post("/ask", c.HomeService.AskQuestion)
		})
		r.Route("/users", func(r chi.Router) {
			r.Post("/", c.UsersService.Create)
			// Auth
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				// Subscriptions
				r.Get("/subscribe", c.UsersService.Subscribe)
				r.Get("/me", c.UsersService.CurrentUser)
				r.Get("/tab-content", c.UsersService.TabContent)
				r.Post("/delete-token", c.UsersService.DeleteToken)
				r.Post("/delete-content", c.UsersService.DeleteAllContent)
				r.Post("/delete-account", c.UsersService.DeleteAccount)
			})
			// Import/export
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Get("/pocket-import", c.ImporterService.PocketImport)
				r.Post("/pocket-import", c.ImporterService.ProcessImport)
				r.Post("/export", c.ImporterService.ProcessExport)
				r.Get("/import-status", c.ImporterService.ImportStatus)
			})
		})
		r.Route("/payments", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Post("/create-checkout-session", c.StripeService.CreateCheckoutSession)
			r.Post("/create-portal-session", c.StripeService.CreatePortalSession)
			r.Get("/portal-session", c.StripeService.GoToBillingPortal)
			r.Get("/success", c.StripeService.Success)
			r.Get("/cancel", c.StripeService.Cancel)
		})

		r.Route("/extension", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", c.ExtensionService.GenerateToken)
		})

		r.Route("/telegram", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", c.TelegramService.RedirectWithAuthToken)
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
				r.Get("/", c.BookmarksService.Index)
				r.Post("/", c.BookmarksService.Create)
				r.Get("/new", c.BookmarksService.New)

				// TODO: Remove when new addon version is released
				r.Get("/{id}/edit", c.BookmarksService.Edit)
				r.Get("/{id}", c.BookmarksService.Edit)

				r.Post("/{id}", c.BookmarksService.Update)
				r.Post("/{id}/delete", c.BookmarksService.Delete)
				r.Get("/{id}/full", c.BookmarksService.GetFullBookmark)
				r.Get("/{id}/markdown", c.BookmarksService.GetBookmarkMarkdown)
				r.Get("/{id}/markdown-content", c.BookmarksService.GetBookmarkMarkdownHTMX)
				r.Post("/{id}/report", c.BookmarksService.ReportBookmark)
			})
		})

		r.Route("/oauth", func(r chi.Router) {
			r.Get("/github", githubService.RedirectToGitHub)
			r.Get("/github/callback", githubService.HandleCallback)

			r.Get("/google", googleService.RedirectToGoogle)
			r.Get("/google/callback", googleService.HandleCallback)
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})
	return r
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("pong"))
}

func LoggerMiddleware(isProduction bool, flow string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t1 := time.Now()
			ctx := r.Context()
			reqLogger := logging.Logger.With(
				"req_path", r.URL.Path,
				"req_method", r.Method,
				"flow", flow,
			)

			if user := usercontext.User(ctx); user != nil {
				reqLogger = reqLogger.With("user", user.ID)
			}
			ctx = loggercontext.WithLogger(ctx, reqLogger)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				reqLogger.Debugw("http request", "from", r.RemoteAddr, "status", ww.Status(), "size", ww.BytesWritten(), "duration", time.Since(t1))
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}
