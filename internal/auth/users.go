package auth

import (
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
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/web"
	"github.com/jackc/pgx/v5"
)

type Users struct {
	Templates struct {
		New             web.Template
		SignIn          web.Template
		ForgotPassword  web.Template
		CheckYourEmail  web.Template
		ResetPassword   web.Template
		UserPage        web.Template
		Token           web.Template
		ProfileTab      web.Template
		TokensTab       web.Template
		ImportExportTab web.Template
	}
	UserService          *models.UserModel
	SessionService       *models.SessionService
	PasswordResetService *models.PasswordResetService
	EmailService         *service.EmailService
	TokenModel           *models.TokenModel
	TurnstileConfig      config.TurnstileConfig
}

func (u Users) New(w http.ResponseWriter, r *http.Request) {
	data := struct {
		TurnstileSiteKey string
	}{
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}
	u.Templates.New.Execute(w, r, data)
}

func (u Users) Create(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")

	var signupTemplateData = struct {
		TurnstileSiteKey string
	}{
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
	logger.Info("create user success")
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) SignIn(w http.ResponseWriter, r *http.Request) {
	var data struct {
		TurnstileSiteKey string
	}
	data.TurnstileSiteKey = u.TurnstileConfig.SiteKey
	u.Templates.SignIn.Execute(w, r, data)
}

func (u Users) ProcessSignIn(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")

	var data struct {
		TurnstileSiteKey string
	}
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
		logger.Info("sign in failed", "error", err)
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
		logger.Info("sign out failed", "error", err)
		http.Error(w, "Sign out failed", http.StatusInternalServerError)
		return
	}
	deleteCookie(w, CookieSession)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	logger := context.Logger(r.Context())
	var data struct {
		Email        string
		IsSubscribed bool
		Tokens       []models.ApiToken
	}
	data.Email = user.Email
	data.IsSubscribed = user.SubscriptionStatus == "premium"
	validTokens, err := u.TokenModel.Get(user.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Info("api token not found for current user")
		} else {
			logger.Info("get api token for current user", "error", err)
			http.Error(w, "Failed to get API token", http.StatusInternalServerError)
			return
		}
	} else {
		data.Tokens = validTokens
	}
	u.Templates.UserPage.Execute(w, r, data)
}

func (u Users) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email string
	}
	data.Email = r.FormValue("email")
	u.Templates.ForgotPassword.Execute(w, r, data)
}

func (u Users) ProcessForgotPassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Email string
	}
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
	// TODO
	resetUrl := "http://localhost:8000/reset-password?" + values.Encode()
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
		Token string
	}
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

func (u Users) GenerateToken(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	w.Header().Set("Content-Type", "text/html")
	type TokenResponse struct {
		APIToken     string
		ErrorMessage string
	}
	user := context.User(r.Context())
	token, err := u.TokenModel.Create(user.ID)
	if err != nil {
		logger.Error("generate token", "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	data := TokenResponse{APIToken: token.Token}
	u.Templates.Token.Execute(w, r, data)
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
	data.IsSubscribed = user.SubscriptionStatus == "premium"

	// Get tokens for tokens tab
	if tab == "tokens" {
		validTokens, err := u.TokenModel.Get(user.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				logger.Info("api token not found for current user")
			} else {
				logger.Info("get api token for current user", "error", err)
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
			log.Printf("read cookie: %v", err)
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
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
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
