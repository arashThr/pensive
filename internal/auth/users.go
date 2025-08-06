package auth

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/logging"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/web"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type Users struct {
	Domain          string
	TurnstileConfig config.TurnstileConfig
	AuthConfig      struct {
		AllowPasswordAuth     bool
		AllowPasswordlessAuth bool
	}
	Templates struct {
		New                    web.Template
		SignIn                 web.Template
		ForgotPassword         web.Template
		CheckYourEmail         web.Template
		ResetPassword          web.Template
		UserPage               web.Template
		Subscribe              web.Template
		Token                  web.Template
		ProfileTab             web.Template
		TokensTab              web.Template
		ImportExportTab        web.Template
		PasswordlessNew        web.Template
		PasswordlessSignIn     web.Template
		PasswordlessCheckEmail web.Template
	}
	UserService          *models.UserModel
	SessionService       *models.SessionService
	PasswordResetService *models.PasswordResetService
	AuthTokenService     *models.AuthTokenService
	EmailService         *service.EmailService
	TokenModel           *models.TokenModel
}

func (u Users) New(w http.ResponseWriter, r *http.Request) {
	// In production, redirect to passwordless signup if password auth is disabled
	if !u.AuthConfig.AllowPasswordAuth && u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/auth/passwordless/signup", http.StatusFound)
		return
	}

	data := struct {
		Title                 string
		TurnstileSiteKey      string
		AllowPasswordlessAuth bool
	}{
		Title:                 "Sign Up",
		TurnstileSiteKey:      u.TurnstileConfig.SiteKey,
		AllowPasswordlessAuth: u.AuthConfig.AllowPasswordlessAuth,
	}
	u.Templates.New.Execute(w, r, data)
}

func (u Users) Create(w http.ResponseWriter, r *http.Request) {
	// Redirect if password auth is disabled
	if !u.AuthConfig.AllowPasswordAuth {
		http.Redirect(w, r, "/auth/passwordless/signup", http.StatusFound)
		return
	}

	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")

	var signupTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Error("turnstile siteverify on sign up", "error", err)
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	user, err := u.UserService.Create(email, password)
	if err != nil {
		if errors.Is(err, errors.ErrEmailTaken) {
			err = errors.Public(err, "That email address is already taken")
		}
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: err.Error(),
			IsError: true,
		})
		return
	}

	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Error("create user failed", "error", err)
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Creating session failed",
			IsError: true,
		})
		return
	}
	setCookie(w, CookieSession, session.Token)
	logger.Infow("create user success")
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) SignIn(w http.ResponseWriter, r *http.Request) {
	// In production, redirect to passwordless signin if password auth is disabled
	if !u.AuthConfig.AllowPasswordAuth && u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/auth/passwordless/signin", http.StatusFound)
		return
	}

	var data struct {
		Title                 string
		TurnstileSiteKey      string
		AllowPasswordlessAuth bool
	}
	data.Title = "Sign In"
	data.TurnstileSiteKey = u.TurnstileConfig.SiteKey
	data.AllowPasswordlessAuth = u.AuthConfig.AllowPasswordlessAuth
	u.Templates.SignIn.Execute(w, r, data)
}

func (u Users) ProcessSignIn(w http.ResponseWriter, r *http.Request) {
	// Redirect if password auth is disabled
	if !u.AuthConfig.AllowPasswordAuth {
		http.Redirect(w, r, "/auth/passwordless/signin", http.StatusFound)
		return
	}

	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")

	var data struct {
		Title            string
		TurnstileSiteKey string
	}
	data.Title = "Sign In"
	data.TurnstileSiteKey = u.TurnstileConfig.SiteKey

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Error("turnstile siteverify on sign in", "error", err)
		u.Templates.SignIn.Execute(w, r, data, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	user, err := u.UserService.Authenticate(email, password)
	if err != nil {
		logger.Infow("sign in failed", "error", err)
		u.Templates.SignIn.Execute(w, r, data, web.NavbarMessage{
			Message: "Email address or password is incorrect",
			IsError: true,
		})
		return
	}
	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Error("sign in process failed", "error", err)
		http.Error(w, "Sign in process failed", http.StatusInternalServerError)
		return
	}
	setCookie(w, CookieSession, session.Token)
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) ProcessSignOut(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	token, err := readCookie(r, CookieSession)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	err = u.SessionService.Delete(token)
	if err != nil {
		logger.Infow("sign out failed", "error", err)
		http.Error(w, "Sign out failed", http.StatusInternalServerError)
		return
	}
	deleteCookie(w, CookieSession)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) Subscribe(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title        string
		IsSubscribed bool
	}
	data.Title = "Subscription"
	user := context.User(r.Context())
	data.IsSubscribed = user.IsSubscriptionPremium()
	u.Templates.Subscribe.Execute(w, r, data)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	logger := context.Logger(r.Context())
	var data struct {
		Title        string
		Email        string
		IsSubscribed bool
		Tokens       []models.ApiToken
	}
	data.Email = user.Email
	data.IsSubscribed = user.IsSubscriptionPremium()
	validTokens, err := u.TokenModel.Get(user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Infow("api token not found for current user")
		} else {
			logger.Infow("get api token for current user", "error", err)
			http.Error(w, "Failed to get API token", http.StatusInternalServerError)
			return
		}
	} else {
		data.Tokens = validTokens
	}
	data.Title = "Account Settings"
	u.Templates.UserPage.Execute(w, r, data)
}

func (u Users) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
		Email string
	}
	data.Title = "Forgot Password"
	data.Email = r.FormValue("email")
	u.Templates.ForgotPassword.Execute(w, r, data)
}

func (u Users) ProcessForgotPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
		Email string
	}
	data.Title = "Check Your Email"
	data.Email = r.FormValue("email")
	pwReset, err := u.PasswordResetService.Create(data.Email)
	if err != nil {
		// TODO: Handle other cases, like when the user does not exist
		log.Println(err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}
	values := url.Values{
		"token": {pwReset.Token},
	}
	resetUrl := fmt.Sprintf("%s/reset-password?", u.Domain) + values.Encode()
	err = u.EmailService.ForgotPassword(data.Email, resetUrl)
	if err != nil {
		log.Println(err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}
	u.Templates.CheckYourEmail.Execute(w, r, data)
}

func (u Users) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
		Token string
	}
	data.Title = "Reset Password"
	data.Token = r.FormValue("token")
	u.Templates.ResetPassword.Execute(w, r, data)
}

func (u Users) ProcessResetPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Token    string
		Password string
	}
	data.Token = r.FormValue("token")
	data.Password = r.FormValue("password")

	user, err := u.PasswordResetService.Consume(data.Token)
	if err != nil {
		// TODO: Better message if failed duo to bad token
		log.Println("consume token:", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}

	err = u.UserService.UpdatePassword(user.ID, data.Password)
	if err != nil {
		log.Printf("update password failed: %v", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}

	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		log.Println("create session for password reset", err)
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	setCookie(w, CookieSession, session.Token)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) DeleteToken(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	tokenId := r.FormValue("token_id")
	user := context.User(r.Context())
	err := u.TokenModel.Delete(user.ID, tokenId)
	if err != nil {
		logger.Error("delete token", "error", err)
		http.Error(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (u Users) TabContent(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	logger := context.Logger(r.Context())

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = r.FormValue("tab")
	}
	if tab == "" {
		tab = "profile"
	}

	var data struct {
		Email        string
		IsSubscribed bool
		Tokens       []models.ApiToken
	}
	data.Email = user.Email
	data.IsSubscribed = user.IsSubscriptionPremium()

	// Get tokens for tokens tab
	if tab == "tokens" {
		validTokens, err := u.TokenModel.Get(user.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				logger.Infow("api token not found for current user")
			} else {
				logger.Infow("get api token for current user", "error", err)
				http.Error(w, "Failed to get API token", http.StatusInternalServerError)
				return
			}
		} else {
			data.Tokens = validTokens
		}
	}

	w.Header().Set("Content-Type", "text/html")

	switch tab {
	case "profile":
		u.Templates.ProfileTab.Execute(w, r, data)
	case "tokens":
		u.Templates.TokensTab.Execute(w, r, data)
	case "import-export":
		u.Templates.ImportExportTab.Execute(w, r, data)
	default:
		u.Templates.ProfileTab.Execute(w, r, data)
	}
}

type UserMiddleware struct {
	SessionService *models.SessionService
}

func (umw UserMiddleware) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := readCookie(r, CookieSession)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := umw.SessionService.User(token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		ctx = context.WithUser(ctx, user)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (umw UserMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.User(r.Context())
		if user == nil {
			http.Redirect(w, r, "/signin", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ApiMiddleware struct {
	TokenModel *models.TokenModel
}

func (amw ApiMiddleware) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			next.ServeHTTP(w, r)
			return
		}
		token := tokenParts[1]
		user, err := amw.TokenModel.User(token)
		if err != nil {
			log.Printf("set user: %v", err)
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		ctx = context.WithUser(ctx, user)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (amw ApiMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := context.User(r.Context())
		if user == nil {
			logging.Logger.Infow("unauthorized request", "remoteAddr", r.RemoteAddr, "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Passwordless Authentication Methods

func (u Users) PasswordlessNew(w http.ResponseWriter, r *http.Request) {
	if !u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/signup", http.StatusFound)
		return
	}

	data := struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}
	u.Templates.PasswordlessNew.Execute(w, r, data)
}

func (u Users) ProcessPasswordlessSignup(w http.ResponseWriter, r *http.Request) {
	if !u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/signup", http.StatusFound)
		return
	}

	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	token := r.FormValue("cf-turnstile-response")

	var signupTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Error("turnstile siteverify on passwordless sign up", "error", err)
		u.Templates.PasswordlessNew.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	// Check if user already exists
	existingUser, err := u.UserService.GetByEmail(email)
	if err == nil && existingUser != nil {
		// User exists, redirect to signin
		u.Templates.PasswordlessNew.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Account already exists. Please check your email for the sign in link.",
			IsError: false,
		})
		// Also send signin link
		u.sendPasswordlessSigninEmail(email, logger)
		return
	}

	// Create auth token for signup
	authToken, err := u.AuthTokenService.Create(email, models.AuthTokenTypeSignup)
	if err != nil {
		logger.Error("create auth token for signup", "error", err)
		u.Templates.PasswordlessNew.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Failed to send sign up email",
			IsError: true,
		})
		return
	}

	// Send signup email asynchronously
	values := url.Values{
		"token": {authToken.Token},
	}
	magicURL := fmt.Sprintf("%s/auth/passwordless/verify?", u.Domain) + values.Encode()

	// Send email in background to avoid blocking the response
	go func() {
		err := u.EmailService.PasswordlessSignup(email, magicURL)
		if err != nil {
			logger.Error("send passwordless signup email", "error", err)
		}
	}()

	// Show check your email page immediately
	var data struct {
		Title string
		Email string
		Type  string
	}
	data.Title = "Check Your Email"
	data.Email = email
	data.Type = "signup"
	u.Templates.PasswordlessCheckEmail.Execute(w, r, data)
}

func (u Users) PasswordlessSignIn(w http.ResponseWriter, r *http.Request) {
	if !u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	data := struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign In",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}
	u.Templates.PasswordlessSignIn.Execute(w, r, data)
}

func (u Users) ProcessPasswordlessSignIn(w http.ResponseWriter, r *http.Request) {
	if !u.AuthConfig.AllowPasswordlessAuth {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}

	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	token := r.FormValue("cf-turnstile-response")

	var signinTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign In",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Error("turnstile siteverify on passwordless sign in", "error", err)
		u.Templates.PasswordlessSignIn.Execute(w, r, signinTemplateData, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	// Check if user exists
	existingUser, err := u.UserService.GetByEmail(email)
	if err != nil || existingUser == nil {
		// User doesn't exist, suggest signup
		u.Templates.PasswordlessSignIn.Execute(w, r, signinTemplateData, web.NavbarMessage{
			Message: "Account not found. Please sign up first.",
			IsError: true,
		})
		return
	}

	// Send signin email asynchronously
	go func() {
		err := u.sendPasswordlessSigninEmail(email, logger)
		if err != nil {
			logger.Error("failed to send passwordless signin email", "error", err)
		}
	}()

	// Show check your email page immediately
	var data struct {
		Title string
		Email string
		Type  string
	}
	data.Title = "Check Your Email"
	data.Email = email
	data.Type = "signin"
	u.Templates.PasswordlessCheckEmail.Execute(w, r, data)
}

func (u Users) VerifyPasswordlessAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := context.Logger(ctx)
	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Error("missing token in passwordless auth verification")
		http.Error(w, "Invalid verification link", http.StatusBadRequest)
		return
	}

	authToken, err := u.AuthTokenService.Consume(token)
	if err != nil {
		logger.Error("consume auth token", "error", err)
		http.Error(w, "Invalid or expired verification link", http.StatusBadRequest)
		return
	}

	var user *models.User

	if authToken.TokenType == models.AuthTokenTypeSignup {
		// Create new user
		user, err = u.createPasswordlessUser(ctx, authToken.Email)
		if err != nil {
			logger.Error("create passwordless user", "error", err)
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
	} else {
		// Get existing user for signin
		user, err = u.UserService.GetByEmail(authToken.Email)
		if err != nil {
			logger.Error("get user for passwordless signin", "error", err)
			http.Error(w, "Account not found", http.StatusNotFound)
			return
		}
	}

	// Create session
	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Error("create session for passwordless auth", "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	setCookie(w, CookieSession, session.Token)
	logger.Infow("passwordless auth success", "email", user.Email, "type", authToken.TokenType)
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) sendPasswordlessSigninEmail(email string, logger *zap.SugaredLogger) error {
	// Create auth token for signin
	authToken, err := u.AuthTokenService.Create(email, models.AuthTokenTypeSignin)
	if err != nil {
		logger.Error("create auth token for signin", "error", err)
		return err
	}

	// Send signin email
	values := url.Values{
		"token": {authToken.Token},
	}
	magicURL := fmt.Sprintf("%s/auth/passwordless/verify?", u.Domain) + values.Encode()
	err = u.EmailService.PasswordlessSignin(email, magicURL)
	if err != nil {
		logger.Error("send passwordless signin email", "error", err)
		return err
	}

	return nil
}

func (u Users) createPasswordlessUser(ctx gocontext.Context, email string) (*models.User, error) {
	// Create user without password (OAuth-style)
	row := u.UserService.Pool.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ($1) RETURNING id, subscription_status, stripe_invoice_id
	`, email)

	user := models.User{
		Email: email,
	}

	err := row.Scan(&user.ID, &user.SubscriptionStatus, &user.StripeInvoiceId)
	if err != nil {
		return nil, fmt.Errorf("create passwordless user: %w", err)
	}

	return &user, nil
}

func validateTurnstileToken(token string, secretKey string, remoteIP string) error {
	if token == "" {
		return fmt.Errorf("turnstile token is required")
	}

	values := url.Values{
		"secret":   {secretKey},
		"response": {token},
		"remoteip": {remoteIP},
	}
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", values)
	if err != nil {
		return fmt.Errorf("turnstile siteverify: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("turnstile siteverify read body: %w", err)
	}

	var turnstileResponse struct {
		Success bool `json:"success"`
	}
	err = json.Unmarshal(body, &turnstileResponse)
	if err != nil {
		return fmt.Errorf("turnstile siteverify unmarshal body: %w", err)
	}

	if !turnstileResponse.Success {
		return fmt.Errorf("turnstile verification failed")
	}

	return nil
}
