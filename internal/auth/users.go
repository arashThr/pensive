package auth

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arashthr/pensive/internal/auth/context/loggercontext"
	"github.com/arashthr/pensive/internal/auth/context/usercontext"
	"github.com/arashthr/pensive/internal/config"
	"github.com/arashthr/pensive/internal/errors"
	"github.com/arashthr/pensive/internal/logging"
	"github.com/arashthr/pensive/internal/models"
	"github.com/arashthr/pensive/internal/service"
	"github.com/arashthr/pensive/web"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type Users struct {
	Domain            string
	TelegramBotName   string
	TurnstileConfig   config.TurnstileConfig
	Templates       struct {
		New                    web.Template
		SignIn                 web.Template
		ForgotPassword         web.Template
		CheckYourEmail         web.Template
		ResetPassword          web.Template
		UserPage               web.Template
		Integrations           web.Template
		Subscribe              web.Template
		Token                  web.Template
		ProfileTab             web.Template
		TokensTab              web.Template
		ImportExportTab        web.Template
		DataManagementTab      web.Template
		PreferencesTab         web.Template
		PasswordlessNew        web.Template
		PasswordlessSignIn     web.Template
		PasswordlessCheckEmail web.Template
	}
	UserService          *models.UserRepo
	SessionService       *models.SessionRepo
	PasswordResetService *models.PasswordResetRepo
	AuthTokenService     *models.AuthTokenService
	EmailService         *service.EmailService
	TokenModel           *models.TokenRepo
	TelegramModel        *models.TelegramRepo
	PodcastScheduleRepo  *models.PodcastScheduleRepo
}

func (u Users) New(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	if user != nil {
		// User is already logged in, redirect to home
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	data := struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}
	u.Templates.New.Execute(w, r, data)
}

func (u Users) Create(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")
	logger.Debugw("sign up attempt", "email", email)

	var signupTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Errorw("turnstile siteverify on sign up", "error", err)
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	user, err := u.UserService.Create(email, password)
	if err != nil {
		if errors.Is(err, errors.ErrEmailTaken) {
			logger.Warnw("sign up failed: email taken", "email", email)
			err = errors.Public(err, "That email address is already taken")
		} else {
			logger.Errorw("sign up failed: user creation error", "error", err, "email", email)
		}
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: err.Error(),
			IsError: true,
		})
		return
	}
	logging.Telegram.SendMessage("New user signed up")

	// For password users, create a session but also send verification email
	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Errorw("create user session failed", "error", err)
		u.Templates.New.Execute(w, r, signupTemplateData, web.NavbarMessage{
			Message: "Creating session failed",
			IsError: true,
		})
		return
	}
	setCookie(w, CookieSession, session.Token)

	// Send verification email asynchronously
	go func() {
		err := u.sendEmailVerification(email)
		if err != nil {
			logger.Errorw("send email verification for new user", "error", err)
		}
	}()

	logger.Infow("create user success, verification email sent", "user_id", user.ID)
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) SignIn(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	if user != nil {
		// User is already logged in, redirect to home
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	var data struct {
		Title            string
		TurnstileSiteKey string
		Next             string
	}
	data.Title = "Sign In"
	data.TurnstileSiteKey = u.TurnstileConfig.SiteKey
	data.Next = r.URL.Query().Get("next")
	u.Templates.SignIn.Execute(w, r, data)
}

func (u Users) ProcessSignIn(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	token := r.FormValue("cf-turnstile-response")
	logger.Debugw("sign in attempt", "email", email)

	var data struct {
		Title            string
		TurnstileSiteKey string
	}
	data.Title = "Sign In"
	data.TurnstileSiteKey = u.TurnstileConfig.SiteKey

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Errorw("turnstile siteverify on sign in", "error", err)
		u.Templates.SignIn.Execute(w, r, data, web.NavbarMessage{
			Message: "Verification failed",
			IsError: true,
		})
		return
	}

	user, err := u.UserService.Authenticate(email, password)
	if err != nil {
		logger.Errorw("sign in failed", "error", err)
		u.Templates.SignIn.Execute(w, r, data, web.NavbarMessage{
			Message: "Email address or password is incorrect",
			IsError: true,
		})
		return
	}
	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Errorw("sign in process failed", "error", err)
		http.Error(w, "Sign in process failed", http.StatusInternalServerError)
		return
	}
	setCookie(w, CookieSession, session.Token)
	logger.Infow("sign in success", "user_id", user.ID)
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusFound)
}

func (u Users) ProcessSignOut(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	user := usercontext.User(r.Context())
	logger.Debugw("sign out attempt", "user_id", user.ID)
	token, err := readCookie(r, CookieSession)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	err = u.SessionService.Delete(token)
	if err != nil {
		logger.Errorw("sign out failed", "error", err)
		http.Error(w, "Sign out failed", http.StatusInternalServerError)
		return
	}
	logger.Infow("sign out success", "user_id", user.ID)
	deleteCookie(w, CookieSession)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) Subscribe(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title        string
		IsSubscribed bool
	}
	data.Title = "Subscription"
	user := usercontext.User(r.Context())
	data.IsSubscribed = user.IsSubscriptionPremium()
	u.Templates.Subscribe.Execute(w, r, data)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
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
			logger.Errorw("get api token for current user", "error", err)
			http.Error(w, "Failed to get API token", http.StatusInternalServerError)
			return
		}
	} else {
		data.Tokens = validTokens
	}
	data.Title = "Account Settings"
	u.Templates.UserPage.Execute(w, r, data)
}

func (u Users) Integrations(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	user := usercontext.User(r.Context())

	var data struct {
		Title           string
		LoggedIn        bool
		Email           string
		TokensCount     int
		TelegramLinked  bool
		TelegramBotName string
		Preferences     *models.SummaryPreferences
	}

	data.Title = "Integrations"
	data.TelegramBotName = u.TelegramBotName
	data.Preferences = &models.SummaryPreferences{
		DailyEnabled:  false,
		DailyHour:     8,
		DailyTimezone: "UTC",
	}

	if user != nil {
		data.LoggedIn = true
		data.Email = user.Email

		if u.UserService != nil {
			prefs, err := u.UserService.GetSummaryPreferences(user.ID)
			if err != nil {
				logger.Errorw("get summary preferences for integrations", "error", err, "user_id", user.ID)
			} else {
				data.Preferences = prefs
			}
		}

		if u.TelegramModel != nil {
			_, err := u.TelegramModel.GetChatIdByUserId(user.ID)
			data.TelegramLinked = err == nil
		}

		if u.TokenModel != nil {
			tokens, err := u.TokenModel.Get(user.ID)
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					logger.Errorw("get api tokens for integrations", "error", err, "user_id", user.ID)
				}
			} else {
				data.TokensCount = len(tokens)
			}
		}
	}

	u.Templates.Integrations.Execute(w, r, data)
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
	logger := loggercontext.Logger(r.Context())
	var data struct {
		Title string
		Email string
	}
	data.Title = "Check Your Email"
	data.Email = r.FormValue("email")
	logger.Debugw("forgot password request", "email", data.Email)
	pwReset, err := u.PasswordResetService.Create(data.Email)
	if err != nil {
		// TODO: Handle other cases, like when the user does not exist
		logger.Errorw("process forgot password", "error", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}
	values := url.Values{
		"token": {pwReset.Token},
	}
	resetUrl := fmt.Sprintf("%s/reset-password?", u.Domain) + values.Encode()
	err = u.EmailService.ForgotPassword(data.Email, resetUrl)
	if err != nil {
		logger.Errorw("reset in forgot password", "error", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}
	logger.Infow("password reset email sent", "email", data.Email)
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
	logger := loggercontext.Logger(r.Context())
	var data struct {
		Token    string
		Password string
	}
	data.Token = r.FormValue("token")
	data.Password = r.FormValue("password")

	user, err := u.PasswordResetService.Consume(data.Token)
	if err != nil {
		// TODO: Better message if failed duo to bad token
		logger.Errorw("consume token for reset password", "error", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}

	err = u.UserService.UpdatePassword(user.ID, data.Password)
	if err != nil {
		logger.Errorw("update password failed", "error", err)
		http.Error(w, "Password reset failed", http.StatusInternalServerError)
		return
	}

	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Errorw("create session for password reset", "error", err)
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	setCookie(w, CookieSession, session.Token)
	logger.Infow("password reset success", "user_id", user.ID)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) DeleteToken(w http.ResponseWriter, r *http.Request) {
	logger := loggercontext.Logger(r.Context())
	tokenId := r.FormValue("token_id")
	user := usercontext.User(r.Context())
	logger.Debugw("delete API token", "user_id", user.ID, "token_id", tokenId)
	err := u.TokenModel.Delete(user.ID, tokenId)
	if err != nil {
		logger.Errorw("delete token", "error", err)
		http.Error(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}
	logger.Infow("API token deleted", "user_id", user.ID, "token_id", tokenId)
	w.WriteHeader(http.StatusOK)
}

func (u Users) TabContent(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = r.FormValue("tab")
	}
	if tab == "" {
		tab = "profile"
	}
	logger.Debugw("tab content", "user_id", user.ID, "tab", tab)

	var data struct {
		Email          string
		IsSubscribed   bool
		Tokens         []models.ApiToken
		Preferences    *models.SummaryPreferences
		TelegramLinked bool
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
				logger.Errorw("get api token for current user", "error", err)
				http.Error(w, "Failed to get API token", http.StatusInternalServerError)
				return
			}
		} else {
			data.Tokens = validTokens
		}
	}

	// Get preferences for preferences tab
	if tab == "preferences" {
		prefs, err := u.UserService.GetSummaryPreferences(user.ID)
		if err != nil {
			logger.Errorw("get summary preferences", "error", err)
			// Use defaults if error
			prefs = &models.SummaryPreferences{
				DailyEnabled:  false,
				DailyHour:     8,
				DailyTimezone: "UTC",
			}
		}
		data.Preferences = prefs

		// Check if user has Telegram linked
		if u.TelegramModel != nil {
			_, err := u.TelegramModel.GetChatIdByUserId(user.ID)
			data.TelegramLinked = err == nil
		}
	}

	w.Header().Set("Content-Type", "text/html")

	switch tab {
	case "profile":
		u.Templates.ProfileTab.Execute(w, r, data)
	case "preferences":
		u.Templates.PreferencesTab.Execute(w, r, data)
	case "tokens":
		u.Templates.TokensTab.Execute(w, r, data)
	case "import-export":
		u.Templates.ImportExportTab.Execute(w, r, data)
	case "data-management":
		u.Templates.DataManagementTab.Execute(w, r, data)
	default:
		u.Templates.ProfileTab.Execute(w, r, data)
	}
}

// SavePreferences handles POST /users/preferences to save weekly and daily podcast preferences.
func (u Users) SavePreferences(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	if err := r.ParseForm(); err != nil {
		logger.Errorw("parse preferences form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	currentPrefs, err := u.UserService.GetSummaryPreferences(user.ID)
	if err != nil {
		logger.Errorw("get current summary preferences", "error", err)
		currentPrefs = &models.SummaryPreferences{
			Enabled:       false,
			Day:           "sunday",
			Email:         true,
			Telegram:      false,
			DailyEnabled:  false,
			DailyHour:     8,
			DailyTimezone: "UTC",
		}
	}

	prefs := *currentPrefs

	// Weekly podcast preferences
	prefs.Enabled = r.FormValue("enabled") == "true"
	prefs.Email = r.FormValue("email") == "true"

	day := r.FormValue("day")
	validDays := map[string]bool{
		"sunday": true, "monday": true, "tuesday": true, "wednesday": true,
		"thursday": true, "friday": true, "saturday": true,
	}
	if validDays[day] {
		prefs.Day = day
	}

	telegramLinked := false
	if u.TelegramModel != nil {
		_, tgErr := u.TelegramModel.GetChatIdByUserId(user.ID)
		telegramLinked = tgErr == nil
	}

	// Telegram delivery is user-editable only when Telegram is linked.
	if telegramLinked {
		prefs.Telegram = r.FormValue("telegram") == "true"
	} else {
		prefs.Telegram = false
	}

	// Daily podcast preferences
	if telegramLinked {
		prefs.DailyEnabled = r.FormValue("daily_enabled") == "true"

		if h, convErr := strconv.Atoi(r.FormValue("daily_hour")); convErr == nil && h >= 0 && h <= 23 {
			prefs.DailyHour = h
		}

		dailyTz := r.FormValue("daily_timezone")
		if dailyTz != "" {
			if _, tzErr := time.LoadLocation(dailyTz); tzErr == nil {
				prefs.DailyTimezone = dailyTz
			}
		}
	}

	if prefs.DailyTimezone == "" {
		prefs.DailyTimezone = "UTC"
	}
	if _, tzErr := time.LoadLocation(prefs.DailyTimezone); tzErr != nil {
		prefs.DailyTimezone = "UTC"
	}

	err = u.UserService.UpdateSummaryPreferences(user.ID, prefs)
	if err != nil {
		logger.Errorw("update summary preferences", "error", err)
		http.Error(w, "Failed to save preferences", http.StatusInternalServerError)
		return
	}

	// Maintain the weekly podcast schedule.
	if prefs.Enabled {
		nextAt := service.NextPublishAt(prefs.Day, 1)
		if schedErr := u.PodcastScheduleRepo.Upsert(user.ID, models.PodcastScheduleTypeWeekly, nextAt); schedErr != nil {
			logger.Errorw("upsert weekly podcast schedule", "error", schedErr)
		}
	} else {
		if schedErr := u.PodcastScheduleRepo.Delete(user.ID, models.PodcastScheduleTypeWeekly); schedErr != nil {
			logger.Errorw("delete weekly podcast schedule", "error", schedErr)
		}
	}

	// Maintain the daily podcast schedule.
	if prefs.DailyEnabled {
		nextAt := service.NextDailyFireAt(prefs.DailyHour, prefs.DailyTimezone)
		if schedErr := u.PodcastScheduleRepo.Upsert(user.ID, models.PodcastScheduleTypeDaily, nextAt); schedErr != nil {
			logger.Errorw("upsert daily podcast schedule", "error", schedErr)
		}
	} else {
		if schedErr := u.PodcastScheduleRepo.Delete(user.ID, models.PodcastScheduleTypeDaily); schedErr != nil {
			logger.Errorw("delete daily podcast schedule", "error", schedErr)
		}
	}

	logger.Infow(
		"saved podcast preferences",
		"user_id", user.ID,
		"enabled", prefs.Enabled,
		"day", prefs.Day,
		"email", prefs.Email,
		"telegram", prefs.Telegram,
		"dailyEnabled", prefs.DailyEnabled,
		"dailyHour", prefs.DailyHour,
		"dailyTimezone", prefs.DailyTimezone,
	)
	w.WriteHeader(http.StatusOK)
}

type UserMiddleware struct {
	SessionService *models.SessionRepo
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
		ctx = usercontext.WithUser(ctx, user)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (umw UserMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := usercontext.User(r.Context())
		if user == nil {
			target := r.URL.RequestURI()
			http.Redirect(w, r, "/signin?next="+url.QueryEscape(target), http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ApiMiddleware struct {
	TokenModel *models.TokenRepo
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
			logging.Logger.Errorw("setting user for api", "error", err)
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		ctx = usercontext.WithUser(ctx, user)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (amw ApiMiddleware) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := usercontext.User(r.Context())
		if user == nil {
			logging.Logger.Infow("unauthorized request", "remoteAddr", r.RemoteAddr, "path", r.URL.Path, "method", r.Method)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// safeNext returns next if it is a safe relative path, otherwise "/home".
// This prevents open redirect attacks.
func safeNext(next string) string {
	if next != "" && strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/home"
}

// DeleteAllContent deletes all user content but keeps the account active
func (u Users) DeleteAllContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	logger.Infow("Content delete requested", "user_id", user.ID)

	// Deleting from library_items will be cascaded to library_content
	_, err := u.UserService.Pool.Exec(ctx, `DELETE FROM library_items WHERE user_id = $1`, user.ID)
	// err = deleteUserContent(ctx, tx)
	if err != nil {
		logger.Errorw("delete user content", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to delete content", http.StatusInternalServerError)
		return
	}

	logger.Infow("user content deleted", "user_id", user.ID, "email", user.Email)

	// Redirect back to user settings page
	http.Redirect(w, r, "/users/me", http.StatusFound)
}

// DeleteAccount deletes the entire user account and all associated data
func (u Users) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	logger.Infow("User delete requested", "user_id", user.ID)

	// Deleting from library_items will be cascaded to library_content
	_, err := u.UserService.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
	// err = deleteUserContent(ctx, tx)
	if err != nil {
		logger.Errorw("delete user", "error", err, "user_id", user.ID)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	logger.Infow("user account deleted", "user_id", user.ID, "email", user.Email)

	// Clear session cookie and redirect to home
	setCookie(w, CookieSession, "")
	http.Redirect(w, r, "/", http.StatusFound)
}

// Passwordless Authentication Methods
func (u Users) PasswordlessNew(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	if user != nil {
		// User is already logged in, redirect to home
		http.Redirect(w, r, "/home", http.StatusFound)
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
	logger := loggercontext.Logger(r.Context())
	email := r.FormValue("email")
	token := r.FormValue("cf-turnstile-response")
	logger.Debugw("passwordless signup attempt", "email", email)

	var signupTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign Up",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Errorw("turnstile siteverify on passwordless sign up", "error", err)
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
		logger.Errorw("create auth token for signup", "error", err)
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
			logger.Errorw("send passwordless signup email", "error", err)
		}
	}()

	// Show check your email page immediately
	var data struct {
		Title string
		Email string
		Type  string
	}
	logger.Infow("passwordless signup email sent", "email", email)
	data.Title = "Check Your Email"
	data.Email = email
	data.Type = "signup"
	u.Templates.PasswordlessCheckEmail.Execute(w, r, data)
}

func (u Users) PasswordlessSignIn(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	if user != nil {
		// User is already logged in, redirect to home
		http.Redirect(w, r, "/home", http.StatusFound)
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
	logger := loggercontext.Logger(r.Context())
	email := r.FormValue("email")
	token := r.FormValue("cf-turnstile-response")
	logger.Debugw("passwordless sign in attempt", "email", email)

	var signinTemplateData = struct {
		Title            string
		TurnstileSiteKey string
	}{
		Title:            "Sign In",
		TurnstileSiteKey: u.TurnstileConfig.SiteKey,
	}

	err := validateTurnstileToken(token, u.TurnstileConfig.SecretKey, r.RemoteAddr)
	if err != nil {
		logger.Errorw("turnstile siteverify on passwordless sign in", "error", err)
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
			logger.Errorw("failed to send passwordless signin email", "error", err)
		}
	}()

	// Show check your email page immediately
	var data struct {
		Title string
		Email string
		Type  string
	}
	logger.Infow("passwordless sign in email sent", "email", email)
	data.Title = "Check Your Email"
	data.Email = email
	data.Type = "signin"
	u.Templates.PasswordlessCheckEmail.Execute(w, r, data)
}

func (u Users) VerifyPasswordlessAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Errorw("missing token in passwordless auth verification")
		http.Error(w, "Invalid verification link", http.StatusBadRequest)
		return
	}

	authToken, err := u.AuthTokenService.Consume(token)
	if err != nil {
		logger.Errorw("consume auth token", "error", err)
		http.Error(w, "Invalid or expired verification link", http.StatusBadRequest)
		return
	}

	var user *models.User

	if authToken.TokenType == models.AuthTokenTypeSignup {
		// Create new user
		user, err = u.createPasswordlessUser(ctx, authToken.Email)
		if err != nil {
			logger.Errorw("create passwordless user", "error", err)
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
			return
		}
	} else {
		// Get existing user for signin
		user, err = u.UserService.GetByEmail(authToken.Email)
		if err != nil {
			logger.Errorw("get user for passwordless signin", "error", err)
			http.Error(w, "Account not found", http.StatusNotFound)
			return
		}
	}

	// Create session
	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Errorw("create session for passwordless auth", "error", err)
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
		logger.Errorw("create auth token for signin", "error", err)
		return err
	}

	// Send signin email
	values := url.Values{
		"token": {authToken.Token},
	}
	magicURL := fmt.Sprintf("%s/auth/passwordless/verify?", u.Domain) + values.Encode()
	err = u.EmailService.PasswordlessSignin(email, magicURL)
	if err != nil {
		logger.Errorw("send passwordless signin email", "error", err)
		return err
	}

	return nil
}

func (u Users) createPasswordlessUser(ctx gocontext.Context, email string) (*models.User, error) {
	logging.Logger.Debugw("creating passwordless user", "email", email)
	// Create user without password, marking email as verified since they used magic link
	row := u.UserService.Pool.QueryRow(ctx, `
		INSERT INTO users (email, email_verified, email_verified_at) 
		VALUES ($1, true, NOW()) 
		RETURNING id, subscription_status, stripe_invoice_id
	`, email)

	user := models.User{
		Email:         email,
		EmailVerified: true,
	}

	err := row.Scan(&user.ID, &user.SubscriptionStatus, &user.StripeInvoiceId)
	if err != nil {
		logging.Logger.Errorw("failed to create passwordless user", "error", err, "email", email)
		return nil, fmt.Errorf("create passwordless user: %w", err)
	}

	logging.Logger.Infow("passwordless user created", "user_id", user.ID, "email", email)
	return &user, nil
}

func (u Users) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	token := r.URL.Query().Get("token")
	if token == "" {
		logger.Errorw("missing token in email verification")
		http.Error(w, "Invalid verification link", http.StatusBadRequest)
		return
	}

	authToken, err := u.AuthTokenService.Consume(token)
	if err != nil {
		logger.Errorw("consume auth token for email verification", "error", err)
		http.Error(w, "Invalid or expired verification link", http.StatusBadRequest)
		return
	}

	if authToken.TokenType != models.AuthTokenTypeEmailVerification {
		logger.Errorw("invalid token type for email verification", "type", authToken.TokenType)
		http.Error(w, "Invalid verification link", http.StatusBadRequest)
		return
	}

	user, err := u.UserService.GetByEmail(authToken.Email)
	if err != nil {
		logger.Errorw("get user for email verification", "error", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.EmailVerified {
		logger.Infow("email already verified", "user_id", user.ID, "email", user.Email)
		http.Redirect(w, r, "/home", http.StatusFound)
		return
	}

	err = u.UserService.MarkEmailVerified(user.ID)
	if err != nil {
		logger.Errorw("mark email verified", "error", err)
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	logger.Infow("email verification success", "user_id", user.ID, "email", user.Email)
	http.Redirect(w, r, "/home", http.StatusFound)
}

func (u Users) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := loggercontext.Logger(ctx)
	user := usercontext.User(ctx)

	if user == nil {
		logger.Warnw("resend verification email: unauthenticated request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if user.EmailVerified {
		logger.Infow("email already verified", "user_id", user.ID)
		http.Error(w, "Email already verified", http.StatusBadRequest)
		return
	}

	err := u.sendEmailVerification(user.Email)
	if err != nil {
		logger.Errorw("resend verification email", "error", err)
		// Return HTML for HTMX
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<div class="p-6 bg-white border-2 border-black">
			<p class="font-bold">FAILED TO SEND EMAIL</p>
			<p class="text-sm">Please try again or contact support.</p>
		</div>`))
		return
	}

	logger.Infow("verification email resent", "user_id", user.ID)
	// Return success HTML for HTMX
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<div class="p-6 bg-white border-2 border-black">
		<p class="font-bold">VERIFICATION EMAIL SENT</p>
		<p class="text-sm">Check your inbox and click the verification link.</p>
	</div>`))
}

func (u Users) sendEmailVerification(email string) error {
	authToken, err := u.AuthTokenService.Create(email, models.AuthTokenTypeEmailVerification)
	if err != nil {
		return fmt.Errorf("create auth token for email verification: %w", err)
	}

	values := url.Values{
		"token": {authToken.Token},
	}
	verificationURL := fmt.Sprintf("%s/auth/verify-email?", u.Domain) + values.Encode()

	err = u.EmailService.EmailVerification(email, verificationURL)
	if err != nil {
		return fmt.Errorf("send email verification: %w", err)
	}

	return nil
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
