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
	"golang.org/x/oauth2/github"
)

type GitHub struct {
	UserService    *models.UserModel
	SessionService *models.SessionService
	Config         *oauth2.Config
	Domain         string
}

type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func NewGitHubOAuth(
	cfg config.GitHubOAuthConfig,
	domain string,
	userService *models.UserModel,
	sessionService *models.SessionService,
) *GitHub {
	redirectURL, err := url.JoinPath(domain, "/oauth/github/callback")
	if err != nil {
		slog.Error("failed to join path", "error", err)
		redirectURL = fmt.Sprintf("%s/oauth/github/callback", domain)
	}

	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     github.Endpoint,
	}

	return &GitHub{
		Config:         config,
		Domain:         domain,
		UserService:    userService,
		SessionService: sessionService,
	}
}

// RedirectToGitHub initiates the OAuth flow by redirecting to GitHub
func (g *GitHub) RedirectToGitHub(w http.ResponseWriter, r *http.Request) {
	// Generate a random state parameter to prevent CSRF attacks
	state, err := rand.String(32)
	if err != nil {
		slog.Error("failed to generate state parameter", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store state in session or cookie for verification
	// For simplicity, we'll use a cookie with a short expiration
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to GitHub OAuth
	authURL := g.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// HandleCallback processes the OAuth callback from GitHub
func (g *GitHub) HandleCallback(w http.ResponseWriter, r *http.Request) {
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

	token, err := g.Config.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("failed to exchange code for token", "error", err)
		http.Error(w, "Failed to authenticate with GitHub", http.StatusInternalServerError)
		return
	}

	// Get user information from GitHub
	githubUser, err := g.getGitHubUser(token.AccessToken)
	if err != nil {
		logger.Error("failed to get GitHub user", "error", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}
	logger.Info("GitHub user", "user", githubUser)

	// Handle user authentication/creation
	user, err := g.authenticateOrCreateUser(githubUser)
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
	logger.Info("GitHub OAuth login successful", "user_id", user.ID, "github_id", githubUser.ID)

	// Redirect to home page
	http.Redirect(w, r, "/home", http.StatusFound)
}

// getGitHubUser fetches user information from GitHub API
func (g *GitHub) getGitHubUser(accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// If email is not public, try to get it from the emails endpoint
	if user.Email == "" {
		user.Email, err = g.getGitHubEmail(accessToken)
		if err != nil {
			return nil, fmt.Errorf("get email: %w", err)
		}
	}

	return &user, nil
}

// getGitHubEmail fetches the user's primary email from GitHub
func (g *GitHub) getGitHubEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("create github email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github email request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github email API returned status: %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode github email response: %w", err)
	}

	// Find the primary verified email
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	// If no primary verified email, return the first verified email
	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("no verified github email found")
}

// authenticateOrCreateUser handles user authentication or creation
func (g *GitHub) authenticateOrCreateUser(githubUser *GitHubUser) (*models.User, error) {
	// Try to find existing user by GitHub OAuth ID
	user, err := g.UserService.GetByOAuth("github", fmt.Sprintf("%d", githubUser.ID))
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
	existingUser, err := g.UserService.GetByEmail(githubUser.Email)
	if err == nil {
		// User exists with this email, link GitHub OAuth to existing account
		err = g.UserService.LinkOAuthToExistingUser(
			existingUser.ID,
			"github",
			fmt.Sprintf("%d", githubUser.ID),
			githubUser.Email,
		)
		if err != nil {
			return nil, fmt.Errorf("link github oauth to existing user: %w", err)
		}
		return existingUser, nil
	}

	if !errors.Is(err, errors.ErrNotFound) {
		// Unexpected error
		return nil, err
	}

	// Create new user with GitHub OAuth
	user, err = g.UserService.CreateOAuthUser(
		"github",
		fmt.Sprintf("%d", githubUser.ID),
		githubUser.Email,
		githubUser.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("authenticate or create user with github oauth: %w", err)
	}

	return user, nil
}
