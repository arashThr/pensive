package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	authcontext "github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/config"
	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/rand"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleAuth struct {
	UserService    *models.UserModel
	SessionService *models.SessionService
	OAuthConfig    *oauth2.Config
	Domain         string
}

type GoogleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

func NewGoogleOAuth(
	cfg config.GoogleOAuthConfig,
	domain string,
	userService *models.UserModel,
	sessionService *models.SessionService,
) *GoogleAuth {
	redirectURL, err := url.JoinPath(domain, "/oauth/google/callback")
	if err != nil {
		slog.Error("failed to join path", "error", err)
		redirectURL = fmt.Sprintf("%s/oauth/google/callback", domain)
	}

	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleAuth{
		UserService:    userService,
		SessionService: sessionService,
		OAuthConfig:    config,
		Domain:         domain,
	}
}

func (g *GoogleAuth) RedirectToGoogle(w http.ResponseWriter, r *http.Request) {
	// Generate a random state parameter to prevent CSRF attacks
	state, err := rand.String(32)
	if err != nil {
		slog.Error("failed to generate state parameter", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store state in cookie for verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to Google OAuth
	authURL := g.OAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (g *GoogleAuth) HandleCallback(w http.ResponseWriter, r *http.Request) {
	logger := authcontext.Logger(r.Context())

	// Verify state parameter
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		logger.Error("missing oauth state cookie", "error", err)
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		logger.Error("oauth state mismatch")
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	// Exchange authorization code for access token
	code := r.URL.Query().Get("code")
	if code == "" {
		logger.Error("missing authorization code")
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := g.OAuthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("failed to exchange code for token", "error", err)
		http.Error(w, "Failed to authenticate with Google", http.StatusInternalServerError)
		return
	}

	// Get user information from Google
	googleUser, err := g.getGoogleUser(token.AccessToken)
	if err != nil {
		logger.Error("failed to get Google user", "error", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	// Handle user authentication/creation
	user, err := g.authenticateOrCreateUser(googleUser)
	if err != nil {
		logger.Error("failed to authenticate or create user", "error", err)
		http.Error(w, "Failed to authenticate user", http.StatusInternalServerError)
		return
	}

	// Create session
	session, err := g.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		logger.Error("failed to create session", "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	setCookie(w, CookieSession, session.Token)
	logger.Info("Google OAuth login successful", "user_id", user.ID, "google_id", googleUser.ID)

	// Redirect to home page
	http.Redirect(w, r, "/home", http.StatusFound)
}

// getGoogleUser fetches user information from Google API
func (g *GoogleAuth) getGoogleUser(accessToken string) (*GoogleUser, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google api returned status: %d", resp.StatusCode)
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &user, nil
}

// authenticateOrCreateUser handles user authentication or creation
func (g *GoogleAuth) authenticateOrCreateUser(googleUser *GoogleUser) (*models.User, error) {
	// Try to find existing user by Google OAuth ID
	user, err := g.UserService.GetByOAuth("google", googleUser.ID)
	if err == nil {
		// User exists, return them
		slog.Info("user exists", "user", user)
		return user, nil
	}

	if !errors.Is(err, errors.ErrNotFound) {
		// Unexpected error
		slog.Error("error for get by oauth", "error", err)
		return nil, err
	}

	// User doesn't exist, try to find by email
	// First, try to get user by email
	existingUser, err := g.UserService.GetByEmail(googleUser.Email)
	if err == nil {
		// User exists with this email, link Google OAuth to existing account
		err = g.UserService.LinkOAuthToExistingUser(
			existingUser.ID,
			"google",
			googleUser.ID,
			googleUser.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("link oauth to existing user: %w", err)
		}
		return existingUser, nil
	}

	if !errors.Is(err, errors.ErrNotFound) {
		// Unexpected error
		return nil, err
	}

	// Create new user with Google OAuth
	user, err = g.UserService.CreateOAuthUser(
		"google",
		googleUser.ID,
		googleUser.Email,
		googleUser.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("create oauth user: %w", err)
	}

	return user, nil
}
