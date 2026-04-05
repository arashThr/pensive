// Example application demonstrating goauth usage.
//
// Run with:
//
//	DATABASE_URL=postgres://postgres:postgres@localhost:5432/goauth_example?sslmode=disable \
//	APP_DOMAIN=http://localhost:8080 \
//	go run .
//
// Email links are printed to stdout (no SMTP required).
package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/arashthr/goauth"
	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/email"
	"github.com/arashthr/goauth/handlers"
	"github.com/arashthr/goauth/migrations"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

var tmpl *template.Template

func main() {
	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/goauth_example?sslmode=disable")
	domain := env("APP_DOMAIN", "http://localhost:8080")
	port := env("PORT", "8080")

	// Database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	// Run migrations
	if err = migrations.Run(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations OK")

	// Parse templates
	tmpl = template.Must(template.New("").ParseFS(templateFS, "templates/*.gohtml"))

	// Build auth
	a := goauth.New(pool, goauth.Config{
		Domain:      domain,
		EmailSender: &email.LogSender{},
		Redirects: struct {
			Success string
			SignOut string
		}{
			Success: "/home",
			SignOut: "/signin",
		},
		Renders: struct {
			SignUpForm              handlers.RenderFunc
			SignInForm              handlers.RenderFunc
			ForgotPwForm           handlers.RenderFunc
			CheckEmail             handlers.RenderFunc
			ResetPwForm            handlers.RenderFunc
			PasswordlessSignUpForm handlers.RenderFunc
			PasswordlessSignInForm handlers.RenderFunc
			PasswordlessCheckEmail handlers.RenderFunc
		}{
			SignUpForm:   renderPage("signup.gohtml"),
			SignInForm:   renderPage("signin.gohtml"),
			ForgotPwForm: renderPage("forgot-password.gohtml"),
			CheckEmail:   renderPage("check-email.gohtml"),
			ResetPwForm:  renderPage("reset-password.gohtml"),
		},
	})

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(a.SessionMiddleware().SetUser)

	// Auth routes
	a.Register(r)

	// App routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusFound)
	})

	r.Get("/home", func(w http.ResponseWriter, r *http.Request) {
		user := usercontext.User(r.Context())
		renderTemplate(w, "home.gohtml", map[string]interface{}{
			"User": user,
		})
	})

	log.Printf("listening on http://localhost:%s\n", port)
	if err = http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

// renderPage returns a RenderFunc that executes the named template.
func renderPage(name string) handlers.RenderFunc {
	return func(w http.ResponseWriter, r *http.Request, data handlers.RenderData) {
		renderTemplate(w, name, data)
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, fmt.Sprintf("template error: %v", err), http.StatusInternalServerError)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
