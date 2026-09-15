// Package goauth is a self-contained, modular authentication library for Go
// web applications.
//
// # Quick start
//
//	pool, _ := pgxpool.New(ctx, dsn)
//
//	a := goauth.New(pool, goauth.Config{
//	    Domain: "https://example.com",
//	    Email: email.Config{Host: "smtp.example.com", Port: 587, ...},
//	    GitHub: goauth.OAuthConfig{ClientID: "...", ClientSecret: "..."},
//	    Google: goauth.OAuthConfig{ClientID: "...", ClientSecret: "..."},
//	})
//
//	r := chi.NewRouter()
//	r.Use(a.SessionMiddleware().SetUser)
//	a.Register(r)
package goauth

import (
	"github.com/arashthr/goauth/email"
	"github.com/arashthr/goauth/handlers"
	oauthpkg "github.com/arashthr/goauth/handlers/oauth"
	"github.com/arashthr/goauth/middleware"
	"github.com/arashthr/goauth/models"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthConfig holds credentials for a single OAuth2 provider.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// TurnstileConfig configures Cloudflare Turnstile CAPTCHA validation.
// Leave TurnstileSecret empty to disable CAPTCHA (useful in development).
type TurnstileConfig struct {
	SiteKey string // forwarded to render data for the client-side widget
	Secret  string // verified server-side; empty = disabled
}

// AdminConfig configures HTTP Basic Auth for admin routes.
type AdminConfig struct {
	Username string
	Password string
}

// Config is the top-level configuration for the Auth object.
type Config struct {
	// Domain is the full base URL (e.g. "https://example.com").
	// Used for building email links and OAuth callback URLs.
	Domain string

	// Email (SMTP) configuration used when EmailSender is nil.
	Email email.Config

	// EmailSender overrides SMTP with a custom sender (e.g. email.LogSender
	// for local development). When nil, an SMTP service is created from Email.
	EmailSender handlers.EmailSender

	// Optional auth methods – leave zero values to disable.
	GitHub    OAuthConfig
	Google    OAuthConfig
	Telegram  string // bot name, e.g. "MyAppBot"
	Turnstile TurnstileConfig
	Admin     AdminConfig

	// Redirects override the default post-auth destinations.
	Redirects struct {
		// After successful sign-up / sign-in. Default: "/"
		Success string
		// After sign-out. Default: "/signin"
		SignOut string
	}

	// RenderFuncs lets the application supply its own template renderer.
	// Each field corresponds to a distinct page.  When nil, a minimal JSON
	// response is returned – useful for API-only apps or during development.
	Renders struct {
		// Password auth
		SignUpForm     handlers.RenderFunc
		SignInForm     handlers.RenderFunc
		ForgotPwForm   handlers.RenderFunc
		CheckEmail     handlers.RenderFunc
		ResetPwForm    handlers.RenderFunc
		// Passwordless auth
		PasswordlessSignUpForm handlers.RenderFunc
		PasswordlessSignInForm handlers.RenderFunc
		PasswordlessCheckEmail handlers.RenderFunc
	}
}

// Auth is the central auth object.  Create one via New, then call Register
// to mount all routes onto a chi router.
type Auth struct {
	Users      *models.UserRepo
	Sessions   *models.SessionRepo
	AuthTokens *models.AuthTokenService
	PwResets   *models.PasswordResetRepo
	ApiTokens  *models.TokenRepo
	Telegram   *models.TelegramRepo
	Email      handlers.EmailSender
	Config     Config
}

// New initialises the Auth object from a database pool and configuration.
// If cfg.EmailSender is set it is used as-is; otherwise an SMTP service is
// created from cfg.Email.
func New(pool *pgxpool.Pool, cfg Config) *Auth {
	var sender handlers.EmailSender
	if cfg.EmailSender != nil {
		sender = cfg.EmailSender
	} else {
		sender = email.NewService(cfg.Email)
	}
	return &Auth{
		Users:      &models.UserRepo{Pool: pool},
		Sessions:   &models.SessionRepo{Pool: pool},
		AuthTokens: models.NewAuthTokenService(pool),
		PwResets:   models.NewPasswordResetRepo(pool),
		ApiTokens:  &models.TokenRepo{Pool: pool},
		Telegram:   &models.TelegramRepo{Pool: pool},
		Email:      sender,
		Config:     cfg,
	}
}

// Register mounts all enabled auth routes onto r.
//
// Routes registered:
//
//	POST   /signup
//	GET    /signup
//	GET    /signin
//	POST   /signin
//	POST   /signout
//	GET    /forgot-password
//	POST   /forgot-password
//	GET    /reset-password
//	POST   /reset-password
//	GET    /auth/passwordless/signup
//	POST   /auth/passwordless/signup
//	GET    /auth/passwordless/signin
//	POST   /auth/passwordless/signin
//	GET    /auth/passwordless/verify
//	GET    /auth/verify-email
//	POST   /auth/resend-verification
//	GET    /extension/auth              (API token for browser extensions)
//	POST   /users/delete-token
//	GET    /oauth/github                (if GitHub credentials set)
//	GET    /oauth/github/callback
//	GET    /oauth/google                (if Google credentials set)
//	GET    /oauth/google/callback
//	GET    /telegram/auth               (if Telegram bot name set)
func (a *Auth) Register(r chi.Router) {
	pwh := a.passwordHandlers()
	plh := a.passwordlessHandlers()
	vh := a.verificationHandlers()
	ath := &handlers.APITokenHandlers{Tokens: a.ApiTokens}

	// Password auth
	r.Get("/signup", pwh.SignUpForm)
	r.Post("/signup", pwh.SignUp)
	r.Get("/signin", pwh.SignInForm)
	r.Post("/signin", pwh.SignIn)
	r.Post("/signout", pwh.SignOut)
	r.Get("/forgot-password", pwh.ForgotPasswordForm)
	r.Post("/forgot-password", pwh.ForgotPassword)
	r.Get("/reset-password", pwh.ResetPasswordForm)
	r.Post("/reset-password", pwh.ResetPassword)

	// Passwordless auth
	r.Get("/auth/passwordless/signup", plh.SignUpForm)
	r.Post("/auth/passwordless/signup", plh.SignUp)
	r.Get("/auth/passwordless/signin", plh.SignInForm)
	r.Post("/auth/passwordless/signin", plh.SignIn)
	r.Get("/auth/passwordless/verify", plh.Verify)

	// Email verification
	r.Get("/auth/verify-email", vh.VerifyEmail)
	r.Post("/auth/resend-verification", vh.ResendVerification)

	// API tokens
	r.Get("/extension/auth", ath.GenerateToken)
	r.Post("/users/delete-token", ath.DeleteToken)

	// GitHub OAuth (optional)
	if a.Config.GitHub.ClientID != "" {
		gh := oauthpkg.NewGitHubOAuth(oauthpkg.GitHubConfig{
			ClientID:        a.Config.GitHub.ClientID,
			ClientSecret:    a.Config.GitHub.ClientSecret,
			Domain:          a.Config.Domain,
			SuccessRedirect: a.Config.Redirects.Success,
		}, a.Users, a.Sessions)
		r.Get("/oauth/github", gh.Redirect)
		r.Get("/oauth/github/callback", gh.Callback)
	}

	// Google OAuth (optional)
	if a.Config.Google.ClientID != "" {
		goog := oauthpkg.NewGoogleOAuth(oauthpkg.GoogleConfig{
			ClientID:        a.Config.Google.ClientID,
			ClientSecret:    a.Config.Google.ClientSecret,
			Domain:          a.Config.Domain,
			SuccessRedirect: a.Config.Redirects.Success,
		}, a.Users, a.Sessions)
		r.Get("/oauth/google", goog.Redirect)
		r.Get("/oauth/google/callback", goog.Callback)
	}

	// Telegram auth (optional)
	if a.Config.Telegram != "" {
		tg := &handlers.TelegramHandler{
			Telegram: a.Telegram,
			Config:   handlers.TelegramConfig{BotName: a.Config.Telegram},
		}
		r.Get("/telegram/auth", tg.Redirect)
	}
}

// SessionMiddleware returns middleware that reads the session cookie and
// injects the user into the request context.
func (a *Auth) SessionMiddleware() *middleware.SessionMiddleware {
	return &middleware.SessionMiddleware{Sessions: a.Sessions}
}

// APIMiddleware returns middleware that reads the Bearer token and injects
// the user into the request context.
func (a *Auth) APIMiddleware() *middleware.APIMiddleware {
	return &middleware.APIMiddleware{Tokens: a.ApiTokens}
}

// AdminMiddleware returns middleware that enforces HTTP Basic Auth.
func (a *Auth) AdminMiddleware() *middleware.AdminMiddleware {
	return &middleware.AdminMiddleware{
		Username: a.Config.Admin.Username,
		Password: a.Config.Admin.Password,
	}
}

// VerificationHandlers returns the email verification handler group so the
// application can call SendVerificationEmail after sign-up.
func (a *Auth) VerificationHandlers() *handlers.VerificationHandlers {
	return a.verificationHandlers()
}

// ----- private builder helpers -----

func (a *Auth) passwordHandlers() *handlers.PasswordHandlers {
	return &handlers.PasswordHandlers{
		Users:    a.Users,
		Sessions: a.Sessions,
		PwResets: a.PwResets,
		Email:    a.Email,
		Config: handlers.PasswordConfig{
			Domain:           a.Config.Domain,
			TurnstileSecret:  a.Config.Turnstile.Secret,
			TurnstileSiteKey: a.Config.Turnstile.SiteKey,
			SuccessRedirect:  a.Config.Redirects.Success,
			SignOutRedirect:  a.Config.Redirects.SignOut,
			RenderSignUp:     a.Config.Renders.SignUpForm,
			RenderSignIn:     a.Config.Renders.SignInForm,
			RenderForgotPw:   a.Config.Renders.ForgotPwForm,
			RenderCheckEmail: a.Config.Renders.CheckEmail,
			RenderResetPw:    a.Config.Renders.ResetPwForm,
		},
	}
}

func (a *Auth) passwordlessHandlers() *handlers.PasswordlessHandlers {
	return &handlers.PasswordlessHandlers{
		Users:      a.Users,
		Sessions:   a.Sessions,
		AuthTokens: a.AuthTokens,
		Email:      a.Email,
		Config: handlers.PasswordlessConfig{
			Domain:           a.Config.Domain,
			TurnstileSecret:  a.Config.Turnstile.Secret,
			TurnstileSiteKey: a.Config.Turnstile.SiteKey,
			SuccessRedirect:  a.Config.Redirects.Success,
			RenderSignUpForm: a.Config.Renders.PasswordlessSignUpForm,
			RenderSignInForm: a.Config.Renders.PasswordlessSignInForm,
			RenderCheckEmail: a.Config.Renders.PasswordlessCheckEmail,
		},
	}
}

func (a *Auth) verificationHandlers() *handlers.VerificationHandlers {
	return &handlers.VerificationHandlers{
		Users:      a.Users,
		AuthTokens: a.AuthTokens,
		Email:      a.Email,
		Config: handlers.VerificationConfig{
			Domain:          a.Config.Domain,
			SuccessRedirect: a.Config.Redirects.Success,
		},
	}
}
