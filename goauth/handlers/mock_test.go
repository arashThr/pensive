package handlers_test

import (
	"context"
	"fmt"

	"github.com/arashthr/goauth/errors"
	"github.com/arashthr/goauth/models"
)

// ----- mockUserStore -----

type mockUserStore struct {
	users map[string]*models.User // keyed by email
	byID  map[models.UserID]*models.User
	byOAuth map[string]*models.User // key: "provider:id"
	nextID  models.UserID
	err    error // if set, all calls return this error
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		users:   make(map[string]*models.User),
		byID:    make(map[models.UserID]*models.User),
		byOAuth: make(map[string]*models.User),
		nextID:  1,
	}
}

func (m *mockUserStore) add(u *models.User) {
	m.users[u.Email] = u
	m.byID[u.ID] = u
}

func (m *mockUserStore) Create(email, password string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	if _, ok := m.users[email]; ok {
		return nil, errors.ErrEmailTaken
	}
	u := &models.User{ID: m.nextID, Email: email, EmailVerified: false}
	m.nextID++
	m.add(u)
	return u, nil
}

func (m *mockUserStore) Get(id models.UserID) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.byID[id]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserStore) GetByEmail(email string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.users[email]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserStore) GetByOAuth(provider, oauthID string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := provider + ":" + oauthID
	u, ok := m.byOAuth[key]
	if !ok {
		return nil, errors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserStore) Authenticate(email, password string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.users[email]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	// For testing, accept any non-empty password if hash is nil (passwordless)
	if u.PasswordHash == nil {
		return nil, fmt.Errorf("no password set")
	}
	// Simple mock: accept password == "correct"
	if password != "correct" {
		return nil, fmt.Errorf("wrong password")
	}
	return u, nil
}

func (m *mockUserStore) UpdatePassword(id models.UserID, password string) error {
	return m.err
}

func (m *mockUserStore) CreateOAuthUser(provider, oauthID, email, oauthEmail string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u := &models.User{
		ID:            m.nextID,
		Email:         email,
		OAuthProvider: &provider,
		OAuthID:       &oauthID,
		EmailVerified: true,
	}
	m.nextID++
	m.add(u)
	m.byOAuth[provider+":"+oauthID] = u
	return u, nil
}

func (m *mockUserStore) CreatePasswordlessUser(_ context.Context, email string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u := &models.User{ID: m.nextID, Email: email, EmailVerified: true}
	m.nextID++
	m.add(u)
	return u, nil
}

func (m *mockUserStore) LinkOAuthToExistingUser(id models.UserID, provider, oauthID, oauthEmail string) error {
	return m.err
}

func (m *mockUserStore) MarkEmailVerified(id models.UserID) error {
	if m.err != nil {
		return m.err
	}
	if u, ok := m.byID[id]; ok {
		u.EmailVerified = true
	}
	return nil
}

func (m *mockUserStore) Delete(id models.UserID) error { return m.err }

// ----- mockSessionStore -----

type mockSessionStore struct {
	sessions map[string]*models.User // token → user
	err      error
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*models.User)}
}

func (m *mockSessionStore) Create(userID models.UserID, ipAddress string) (*models.Session, error) {
	if m.err != nil {
		return nil, m.err
	}
	tok := fmt.Sprintf("tok_%d", userID)
	s := &models.Session{Token: tok, UserID: userID}
	return s, nil
}

func (m *mockSessionStore) User(token string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.sessions[token]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return u, nil
}

func (m *mockSessionStore) Delete(token string) error {
	delete(m.sessions, token)
	return m.err
}

// ----- mockPasswordResetStore -----

type mockPasswordResetStore struct {
	tokens map[string]*models.PasswordReset
	users  map[string]*models.User // email → user
	err    error
}

func newMockPasswordResetStore() *mockPasswordResetStore {
	return &mockPasswordResetStore{
		tokens: make(map[string]*models.PasswordReset),
		users:  make(map[string]*models.User),
	}
}

func (m *mockPasswordResetStore) Create(email string) (*models.PasswordReset, error) {
	if m.err != nil {
		return nil, m.err
	}
	pr := &models.PasswordReset{Token: "reset_" + email}
	m.tokens[pr.Token] = pr
	return pr, nil
}

func (m *mockPasswordResetStore) Consume(token string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	pr, ok := m.tokens[token]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	_ = pr
	return &models.User{ID: 1, Email: "user@example.com"}, nil
}

// ----- mockAuthTokenStore -----

type mockAuthTokenStore struct {
	tokens map[string]*models.AuthToken
	err    error
}

func newMockAuthTokenStore() *mockAuthTokenStore {
	return &mockAuthTokenStore{tokens: make(map[string]*models.AuthToken)}
}


func (m *mockAuthTokenStore) Create(email string, tokenType models.AuthTokenType) (*models.AuthToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	tok := "at_" + email + "_" + string(tokenType)
	at := &models.AuthToken{
		Token:     tok,
		Email:     email,
		TokenType: tokenType,
	}
	m.tokens[tok] = at
	return at, nil
}

func (m *mockAuthTokenStore) Consume(token string) (*models.AuthToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	at, ok := m.tokens[token]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	delete(m.tokens, token)
	return at, nil
}

// ----- mockEmailSender -----

type mockEmailSender struct {
	sent []string
	err  error
}

func (m *mockEmailSender) ForgotPassword(to, resetURL string) error {
	m.sent = append(m.sent, "forgot:"+to)
	return m.err
}
func (m *mockEmailSender) PasswordlessSignup(to, magicURL string) error {
	m.sent = append(m.sent, "pl_signup:"+to)
	return m.err
}
func (m *mockEmailSender) PasswordlessSignin(to, magicURL string) error {
	m.sent = append(m.sent, "pl_signin:"+to)
	return m.err
}
func (m *mockEmailSender) EmailVerification(to, verificationURL string) error {
	m.sent = append(m.sent, "verify:"+to)
	return m.err
}

// ----- mockTokenStore -----

type mockTokenStore struct {
	err error
}

func (m *mockTokenStore) Create(userID models.UserID, source string) (*models.GeneratedApiToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &models.GeneratedApiToken{
		ApiToken: models.ApiToken{ID: 1, UserID: userID, TokenSource: source},
		Token:    "apitok_" + fmt.Sprint(userID),
	}, nil
}

func (m *mockTokenStore) Get(userID models.UserID) ([]models.ApiToken, error) {
	return nil, m.err
}

func (m *mockTokenStore) Delete(userID models.UserID, tokenID string) error {
	return m.err
}
