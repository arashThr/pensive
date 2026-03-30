package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/goauth/cookie"
	"github.com/arashthr/goauth/errors"
	"github.com/arashthr/goauth/models"
	"github.com/arashthr/goauth/rand"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleConfig configures the Google OAuth handler.
type GoogleConfig struct {
	ClientID        string
	ClientSecret    string
	Domain          string // base URL used to build the callback URL
	SuccessRedirect string // default: "/"
}

// GoogleOAuth handles Google OAuth2 authentication.
type GoogleOAuth struct {
	Users    UserStore
	Sessions SessionStore
	Config   GoogleConfig
	oauth    *oauth2.Config
}

// NewGoogleOAuth creates a GoogleOAuth handler with routes mounted at /oauth/google.
func NewGoogleOAuth(cfg GoogleConfig, users UserStore, sessions SessionStore) *GoogleOAuth {
	callbackURL, err := url.JoinPath(cfg.Domain, "/oauth/google/callback")
	if err != nil {
		callbackURL = cfg.Domain + "/oauth/google/callback"
	}
	return &GoogleOAuth{
		Users:    users,
		Sessions: sessions,
		Config:   cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  callbackURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// Redirect starts the Google OAuth flow (GET /oauth/google).
func (h *GoogleOAuth) Redirect(w http.ResponseWriter, r *http.Request) {
	l := log(r)
	state, err := rand.String(32)
	if err != nil {
		l.Errorw("google oauth – generate state failed", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, oauthStateCookie(state))
	http.Redirect(w, r, h.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline), http.StatusTemporaryRedirect)
}

// Callback handles the Google OAuth callback (GET /oauth/google/callback).
func (h *GoogleOAuth) Callback(w http.ResponseWriter, r *http.Request) {
	l := log(r)

	if err := verifyOAuthState(w, r); err != nil {
		l.Warnw("google oauth – state mismatch", "error", err)
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
		l.Errorw("google oauth – token exchange failed", "error", err)
		http.Error(w, "Failed to authenticate with Google.", http.StatusInternalServerError)
		return
	}

	gUser, err := fetchGoogleUser(token.AccessToken)
	if err != nil {
		l.Errorw("google oauth – fetch user failed", "error", err)
		http.Error(w, "Failed to get user information.", http.StatusInternalServerError)
		return
	}
	l.Infow("google oauth – user fetched", "google_id", gUser.ID, "email", gUser.Email)

	user, err := h.authenticateOrCreate(gUser)
	if err != nil {
		l.Errorw("google oauth – authenticate/create failed", "error", err)
		http.Error(w, "Failed to authenticate user.", http.StatusInternalServerError)
		return
	}

	session, err := h.Sessions.Create(user.ID, r.RemoteAddr)
	if err != nil {
		l.Errorw("google oauth – session create failed", "error", err)
		http.Error(w, "Failed to create session.", http.StatusInternalServerError)
		return
	}
	cookie.Set(w, cookie.SessionCookieName, session.Token)
	l.Infow("google oauth – login success", "user_id", user.ID)
	http.Redirect(w, r, redirectOrDefault(h.Config.SuccessRedirect, "/"), http.StatusFound)
}

func (h *GoogleOAuth) authenticateOrCreate(gUser *googleUser) (*models.User, error) {
	user, err := h.Users.GetByOAuth("google", gUser.ID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, errors.ErrNotFound) {
		return nil, fmt.Errorf("google oauth get by oauth: %w", err)
	}

	existing, err := h.Users.GetByEmail(gUser.Email)
	if err == nil {
		if linkErr := h.Users.LinkOAuthToExistingUser(existing.ID, "google", gUser.ID, gUser.Email); linkErr != nil {
			return nil, fmt.Errorf("google oauth link user: %w", linkErr)
		}
		return existing, nil
	}
	if !errors.Is(err, errors.ErrNotFound) {
		return nil, fmt.Errorf("google oauth get by email: %w", err)
	}

	return h.Users.CreateOAuthUser("google", gUser.ID, gUser.Email, gUser.Email)
}

// ----- Google API helpers -----

type googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

func fetchGoogleUser(accessToken string) (*googleUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("google user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google user request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google user API returned %d", resp.StatusCode)
	}
	var u googleUser
	if err = json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("google user decode: %w", err)
	}
	return &u, nil
}
