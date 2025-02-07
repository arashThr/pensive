package controllers

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/errors"
	"github.com/arashthr/go-course/models"
)

type Users struct {
	Templates struct {
		New            Template
		SignIn         Template
		ForgotPassword Template
		CheckYourEmail Template
		ResetPassword  Template
		UserPage       Template
	}
	UserService          *models.UserService
	SessionService       *models.SessionService
	PasswordResetService *models.PasswordResetService
	EmailService         *models.EmailService
	ApiService           *models.ApiService
}

func (u Users) New(w http.ResponseWriter, r *http.Request) {
	u.Templates.New.Execute(w, r, nil)
}

func (u Users) Create(w http.ResponseWriter, r *http.Request) {
	log.Println("create user request")
	var data struct {
		Email    string
		Password string
	}
	data.Email = r.FormValue("email")
	data.Password = r.FormValue("password")

	user, err := u.UserService.Create(data.Email, data.Password)
	if err != nil {
		if errors.Is(err, models.ErrEmailTaken) {
			err = errors.Public(err, "That email address is already taken")
		}
		u.Templates.New.Execute(w, r, data, err)
		return
	}

	session, err := u.SessionService.Create(user.ID)
	if err != nil {
		log.Println(err)
		u.Templates.New.Execute(w, r, data, errors.Public(err, "Creating session failed"))
		return
	}
	setCookie(w, CookieSession, session.Token)
	log.Println("create user success")
	http.Redirect(w, r, "/users/me", http.StatusFound)
}

func (u Users) SignIn(w http.ResponseWriter, r *http.Request) {
	u.Templates.SignIn.Execute(w, r, nil)
}

func (u Users) ProcessSignIn(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	user, err := u.UserService.Authenticate(email, password)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}
	session, err := u.SessionService.Create(user.ID)
	if err != nil {
		log.Print(err)
		http.Error(w, "Sign in process failed", http.StatusInternalServerError)
		return
	}
	setCookie(w, CookieSession, session.Token)
	http.Redirect(w, r, "/users/me", http.StatusFound)
}

func (u Users) ProcessSignOut(w http.ResponseWriter, r *http.Request) {
	token, err := readCookie(r, CookieSession)
	if err != nil {
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	err = u.SessionService.Delete(token)
	if err != nil {
		log.Printf("process sign out: %v\n", err)
		http.Error(w, "Sign out failed", http.StatusInternalServerError)
		return
	}
	deleteCookie(w, CookieSession)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) CurrentUser(w http.ResponseWriter, r *http.Request) {
	user := context.User(r.Context())
	var data struct {
		Email string
	}
	data.Email = user.Email
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

	session, err := u.SessionService.Create(user.ID)
	if err != nil {
		log.Println("create session for password reset", err)
		http.Redirect(w, r, "/signin", http.StatusFound)
		return
	}
	setCookie(w, CookieSession, session.Token)
	http.Redirect(w, r, "/signin", http.StatusFound)
}

func (u Users) GenerateToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user := context.User(r.Context())
	token, err := u.ApiService.Create(user.ID)
	if err != nil {
		log.Printf("create api token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `{"apiToken": "%s"}`, token.Token)
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
		log.Printf("setting user in context: %v", user.ID)
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
	ApiService *models.ApiService
}

func (amw ApiMiddleware) SetUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		token := tokenParts[1]
		user, err := amw.ApiService.User(token)
		if err != nil {
			log.Printf("api user: %v", err)
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
