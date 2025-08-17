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

	// Models
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
	tokenModel := &models.TokenRepo{
		Pool: pool,
	}
	stripeModel := models.NewStripeRepo(cfg.Stripe.Key, pool)
	bookmarksModel := &models.BookmarkRepo{
		Pool:        pool,
		GenAIClient: genAIClient,
	}
	telegramModel := &models.TelegramRepo{
		Pool: pool,
	}
	importJobModel := &models.ImportJobRepo{
		Pool: pool,
	}
	authTokenService := models.NewAuthTokenRepo(pool)

	// Middlewares
	umw := auth.UserMiddleware{
		SessionService: sessionRepo,
	}
	amw := auth.ApiMiddleware{
		TokenModel: tokenModel,
	}
	csrfMw := csrf.Protect(
		[]byte(cfg.CSRF.Key),
		csrf.Secure(cfg.CSRF.Secure),
		csrf.Path("/"),
	)

	// Services
	emailService := service.NewEmailService(cfg.SMTP)

	usersService := auth.Users{
		Domain:               cfg.Domain,
		TurnstileConfig:      cfg.Turnstile,
		UserService:          userRepo,
		SessionService:       sessionRepo,
		PasswordResetService: passwordResetRepo,
		AuthTokenService:     authTokenService,
		EmailService:         emailService,
		TokenModel:           tokenModel,
	}

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
		BookmarkModel: bookmarksModel,
	}
	bookmarksService.Templates.New = views.Must(views.ParseTemplate("bookmarks/new.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.Edit = views.Must(views.ParseTemplate("bookmarks/edit.gohtml", "tailwind.gohtml", "bookmarks/markdown.gohtml"))
	bookmarksService.Templates.Index = views.Must(views.ParseTemplate("bookmarks/index.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.Markdown = views.Must(views.ParseTemplate("bookmarks/markdown.gohtml", "tailwind.gohtml"))
	bookmarksService.Templates.MarkdownNotAvailable = views.Must(views.ParseTemplate("bookmarks/markdown-not-available.gohtml", "tailwind.gohtml"))

	homeService := service.Home{
		BookmarkModel: bookmarksModel,
	}
	homeService.Templates.Home = views.Must(views.ParseTemplate("home/home.gohtml", "tailwind.gohtml", "home/recent-results.gohtml"))
	homeService.Templates.SearchResults = views.Must(views.ParseTemplate("home/search-results.gohtml"))
	homeService.Templates.RecentResults = views.Must(views.ParseTemplate("home/recent-results.gohtml"))

	importerService := service.Importer{
		ImportJobModel: importJobModel,
		BookmarkModel:  bookmarksModel,
	}
	importerService.Templates.PocketImport = views.Must(views.ParseTemplate("user/pocket-import.gohtml", "tailwind.gohtml"))
	importerService.Templates.ImportProcessing = views.Must(views.ParseTemplate("user/import-processing.gohtml", "tailwind.gohtml"))
	importerService.Templates.ImportStatus = views.Must(views.ParseTemplate("user/import-status.gohtml", "tailwind.gohtml"))

	userService := service.User{
		BookmarkModel:    bookmarksModel,
		AuthTokenService: authTokenService,
		EmailService:     emailService,
		Domain:           cfg.Domain,
	}

	apiService := service.Api{
		BookmarkModel: bookmarksModel,
	}

	tokenService := service.Token{
		TokenModel: tokenModel,
	}

	stripService := service.Stripe{
		Domain:              cfg.Domain,
		PriceId:             cfg.Stripe.PriceId,
		StripeWebhookSecret: cfg.Stripe.StripeWebhookSecret,
		StripeModel:         stripeModel,
	}
	stripService.Templates.Success = views.Must(views.ParseTemplate("payments/success.gohtml", "tailwind.gohtml"))
	stripService.Templates.Cancel = views.Must(views.ParseTemplate("payments/cancel.gohtml", "tailwind.gohtml"))

	extensionService := auth.Extension{
		TokenModel: tokenModel,
	}

	telegramService := auth.Telegram{
		TelegramModel: telegramModel,
		BotName:       cfg.Telegram.BotName,
	}

	// OAuth services
	githubService := auth.NewGitHubOAuth(cfg.GitHub, cfg.Domain, userRepo, sessionRepo)
	googleService := auth.NewGoogleOAuth(cfg.Google, cfg.Domain, userRepo, sessionRepo)

	// Start import processor in background
	importProcessor := importer.ImportProcessor{
		ImportJobModel: importJobModel,
		BookmarkModel:  bookmarksModel,
		UserModel:      userRepo,
	}
	go importProcessor.Start(ctx)

	// Middlewares
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Use(amw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production", "api"))

		r.Get("/ping", healthCheck)
		r.Post("/stripe-webhooks", stripService.Webhook)

		r.Route("/v1", func(r chi.Router) {
			r.Use(amw.RequireUser)
			// TODO: Remove this
			r.Get("/ping", tokenService.AuthenticatedPing)
			r.Route("/tokens", func(r chi.Router) {
				r.Delete("/current", tokenService.DeleteToken)
			})
			r.Route("/user", func(r chi.Router) {
				r.Get("/", userService.CurrentUserAPI)
				r.Post("/request-verification", userService.RequestVerificationEmailAPI)
			})
			r.Route("/bookmarks", func(r chi.Router) {
				r.Get("/", apiService.IndexAPI)
				r.Post("/", apiService.CreateAPI)
				r.Delete("/", apiService.DeleteByLinkAPI)
				r.Get("/check", apiService.CheckBookmarkByLinkAPI)
				r.Get("/{id}", apiService.GetAPI)
				r.Put("/{id}", apiService.UpdateAPI)
				r.Delete("/{id}", apiService.DeleteAPI)
				r.Get("/search", apiService.SearchAPI)
			})
		})
	})

	// Web routes
	r.Group(func(r chi.Router) {
		r.Use(umw.SetUser)
		r.Use(LoggerMiddleware(cfg.Environment == "production", "web"))
		r.Use(csrfMw)

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			user := usercontext.User(r.Context())
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
		r.Get("/signup", usersService.New)
		r.Get("/signin", usersService.SignIn)
		r.Post("/signin", usersService.ProcessSignIn)
		r.Post("/signout", usersService.ProcessSignOut)
		r.Get("/forgot-pw", usersService.ForgotPassword)
		r.Post("/forgot-pw", usersService.ProcessForgotPassword)
		r.Get("/reset-password", usersService.ResetPassword)
		r.Post("/reset-password", usersService.ProcessResetPassword)

		// Email verification routes
		r.Route("/auth", func(r chi.Router) {
			r.Get("/verify-email", usersService.VerifyEmail)

			r.Route("/resend-verification", func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Post("/", usersService.ResendVerificationEmail)
			})

			r.Route("/passwordless", func(r chi.Router) {
				r.Get("/signup", usersService.PasswordlessNew)
				r.Post("/signup", usersService.ProcessPasswordlessSignup)
				r.Get("/signin", usersService.PasswordlessSignIn)
				r.Post("/signin", usersService.ProcessPasswordlessSignIn)
				r.Get("/verify", usersService.VerifyPasswordlessAuth)
			})
		})
		r.Route("/home", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/", homeService.Index)
			r.Get("/search", homeService.Search)
		})
		r.Route("/users", func(r chi.Router) {
			r.Post("/", usersService.Create)
			// Auth
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				// Subscriptions
				r.Get("/subscribe", usersService.Subscribe)
				r.Get("/me", usersService.CurrentUser)
				r.Get("/tab-content", usersService.TabContent)
				r.Post("/delete-token", usersService.DeleteToken)
				r.Post("/delete-content", usersService.DeleteAllContent)
				r.Post("/delete-account", usersService.DeleteAccount)
			})
			// Import/export
			r.Group(func(r chi.Router) {
				r.Use(umw.RequireUser)
				r.Get("/pocket-import", importerService.PocketImport)
				r.Post("/pocket-import", importerService.ProcessImport)
				r.Post("/export", importerService.ProcessExport)
				r.Get("/import-status", importerService.ImportStatus)
			})
		})
		r.Route("/payments", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Post("/create-checkout-session", stripService.CreateCheckoutSession)
			r.Post("/create-portal-session", stripService.CreatePortalSession)
			r.Get("/portal-session", stripService.GoToBillingPortal)
			r.Get("/success", stripService.Success)
			r.Get("/cancel", stripService.Cancel)
		})

		r.Route("/extension", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", extensionService.GenerateToken)
		})

		r.Route("/telegram", func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/auth", telegramService.RedirectWithAuthToken)
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
				r.Get("/", bookmarksService.Index)
				r.Post("/", bookmarksService.Create)
				r.Get("/new", bookmarksService.New)
				r.Get("/{id}/edit", bookmarksService.Edit)
				r.Post("/{id}", bookmarksService.Update)
				r.Post("/{id}/delete", bookmarksService.Delete)
				r.Get("/{id}/full", bookmarksService.GetFullBookmark)
				r.Get("/{id}/markdown", bookmarksService.GetBookmarkMarkdown)
				r.Get("/{id}/markdown-content", bookmarksService.GetBookmarkMarkdownHTMX)
				r.Post("/{id}/report", bookmarksService.ReportBookmark)
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

	fmt.Printf("Starting server on %s...", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, r)
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
