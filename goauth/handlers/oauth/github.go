package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/goauth/cookie"
	"github.com/arashthr/goauth/errors"
	"github.com/arashthr/goauth/models"
	"golang.org/x/oauth2"
	ghendpoint "golang.org/x/oauth2/github"
)

// GitHubConfig configures the GitHub OAuth handler.
type GitHubConfig struct {
	ClientID        string
	ClientSecret    string
	Domain          string // base URL used to build the callback URL
	SuccessRedirect string // default: "/"
}

// GitHubOAuth handles GitHub OAuth2 authentication.
type GitHubOAuth struct {
	Users    UserStore
	Sessions SessionStore
	Config   GitHubConfig
	oauth    *oauth2.Config
}

// NewGitHubOAuth creates a GitHubOAuth handler with routes mounted at /oauth/github.
func NewGitHubOAuth(cfg GitHubConfig, users UserStore, sessions SessionStore) *GitHubOAuth {
	callbackURL, err := url.JoinPath(cfg.Domain, "/oauth/github/callback")
	if err != nil {
		callbackURL = cfg.Domain + "/oauth/github/callback"
	}
	return &GitHubOAuth{
		Users:    users,
		Sessions: sessions,
		Config:   cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  callbackURL,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     ghendpoint.Endpoint,
		},
	}
}

// Redirect starts the GitHub OAuth flow (GET /oauth/github).
func (h *GitHubOAuth) Redirect(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	state, err := generateState()
	if err != nil {
		l.Errorw("github oauth – generate state failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, oauthStateCookie(state))
	http.Redirect(w, r, h.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline), http.StatusTemporaryRedirect)
}

// Callback handles the GitHub OAuth callback (GET /oauth/github/callback).
func (h *GitHubOAuth) Callback(w http.ResponseWriter, r *http.Request) {
	l := log(r)

	if err := verifyOAuthState(w, r); err != nil {
		l.Warnw("github oauth – state mismatch", "error", err)
		http.Error(w, "Invalid OAuth state.", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code.", http.StatusBadRequest)
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		l.Errorw("github oauth – token exchange failed", "error", err)
		http.Error(w, "Failed to authenticate with GitHub.", http.StatusInternalServerError)
		return
	}

	ghUser, err := fetchGitHubUser(token.AccessToken)
	if err != nil {
		l.Errorw("github oauth – fetch user failed", "error", err)
		http.Error(w, "Failed to get user information.", http.StatusInternalServerError)
		return
	}
	l.Infow("github oauth – user fetched", "github_id", ghUser.ID, "email", ghUser.Email)

	user, err := h.authenticateOrCreate(ghUser)
	if err != nil {
		l.Errorw("github oauth – authenticate/create failed", "error", err)
		http.Error(w, "Failed to authenticate user.", http.StatusInternalServerError)
		return
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("github oauth – session create failed", "error", err)
		http.Error(w, "Failed to create session.", http.StatusInternalServerError)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("github oauth – login success", "user_id", user.ID)
	http.Redirect(w, r, redirectOrDefault(h.Config.SuccessRedirect, "/"), http.StatusFound)
}

func (h *GitHubOAuth) authenticateOrCreate(ghUser *githubUser) (*models.User, error) {
	providerID := fmt.Sprintf("%d", ghUser.ID)

	user, err := h.Users.GetByOAuth("github", providerID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, errors.ErrNotFound) {
		return nil, fmt.Errorf("github oauth get by oauth: %w", err)
	}

	existing, err := h.Users.GetByEmail(ghUser.Email)
	if err == nil {
		if linkErr := h.Users.LinkOAuthToExistingUser(existing.ID, "github", providerID, ghUser.Email); linkErr != nil {
			return nil, fmt.Errorf("github oauth link user: %w", linkErr)
		}
		return existing, nil
	}
	if !errors.Is(err, errors.ErrNotFound) {
		return nil, fmt.Errorf("github oauth get by email: %w", err)
	}

	return h.Users.CreateOAuthUser("github", providerID, ghUser.Email, ghUser.Email)
}

// ----- GitHub API helpers -----

type githubUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func fetchGitHubUser(accessToken string) (*githubUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user API returned %d", resp.StatusCode)
	}
	var u githubUser
	if err = json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}
	if u.Email == "" {
		u.Email, err = fetchGitHubPrimaryEmail(accessToken)
		if err != nil {
			return nil, err
		}
	}
	return &u, nil
}

func fetchGitHubPrimaryEmail(accessToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", fmt.Errorf("github emails request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github emails request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails API returned %d", resp.StatusCode)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("github emails decode: %w", err)
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", fmt.Errorf("no verified GitHub email found")
}
