package auth

import (
	"archive/zip"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/arashthr/go-course/internal/auth/context"
	"github.com/arashthr/go-course/internal/errors"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/internal/service"
	"github.com/arashthr/go-course/web"
	"github.com/jackc/pgx/v5"
)

type Users struct {
	Templates struct {
		New              web.Template
		SignIn           web.Template
		ForgotPassword   web.Template
		CheckYourEmail   web.Template
		ResetPassword    web.Template
		UserPage         web.Template
		Token            web.Template
		ImportExport     web.Template
		ImportProcessing web.Template
		ImportStatus     web.Template
	}
	UserService          *models.UserModel
	SessionService       *models.SessionService
	PasswordResetService *models.PasswordResetService
	EmailService         *service.EmailService
	TokenModel           *models.TokenModel
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
		if errors.Is(err, errors.ErrEmailTaken) {
			err = errors.Public(err, "That email address is already taken")
		}
		u.Templates.New.Execute(w, r, data, web.NavbarMessage{
			Message: err.Error(),
			IsError: true,
		})
		return
	}

	session, err := u.SessionService.Create(user.ID, r.RemoteAddr)
	if err != nil {
		log.Println(err)
		u.Templates.New.Execute(w, r, data, web.NavbarMessage{
			Message: "Creating session failed",
			IsError: true,
		})
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
	logger := context.Logger(r.Context())
	if err != nil {
		logger.Info("sign in failed", "error", err)
		u.Templates.SignIn.Execute(w, r, nil, web.NavbarMessage{
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
	http.Redirect(w, r, "/", http.StatusFound)
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

// ImportExport displays the import/export page
func (u Users) ImportExport(w http.ResponseWriter, r *http.Request) {
	u.Templates.ImportExport.Execute(w, r, nil)
}

// ProcessImport handles the file upload and initial validation
func (u Users) ProcessImport(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())

	// Parse multipart form (8MB max size)
	err := r.ParseMultipartForm(8 << 20)
	if err != nil {
		logger.Error("parse multipart form", "error", err)
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("import-file")
	if err != nil {
		logger.Error("get form file", "error", err)
		http.Error(w, "Failed to get uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		http.Error(w, "Only ZIP files are supported", http.StatusBadRequest)
		return
	}

	// Get form values
	source := r.FormValue("source")
	importOption := r.FormValue("import-option")

	// Basic validation
	if source != "pocket" {
		http.Error(w, "Unsupported import source", http.StatusBadRequest)
		return
	}

	if importOption != "highlighted" && importOption != "all" {
		http.Error(w, "Invalid import option", http.StatusBadRequest)
		return
	}

	// Create a temporary file to store the uploaded ZIP
	tempFile, err := os.CreateTemp("", "pocket-import-*.zip")
	if err != nil {
		logger.Error("create temp file", "error", err)
		http.Error(w, "Failed to process upload", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up temp file
	defer tempFile.Close()

	// Copy uploaded file to temp file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		logger.Error("copy file to temp", "error", err)
		http.Error(w, "Failed to process upload", http.StatusInternalServerError)
		return
	}

	// Basic ZIP validation - try to open it
	_, err = zip.OpenReader(tempFile.Name())
	if err != nil {
		logger.Error("validate zip file", "error", err)
		http.Error(w, "Invalid ZIP file", http.StatusBadRequest)
		return
	}

	logger.Info("import validation successful",
		"user_id", user.ID,
		"source", source,
		"option", importOption,
		"filename", header.Filename,
		"size", header.Size)

	// For now, just redirect to processing page
	// In a real implementation, you would:
	// 1. Store the file in persistent storage
	// 2. Queue a background job for processing
	// 3. Generate a job ID for tracking

	data := struct {
		Source string
		JobID  string
	}{
		Source: strings.ToUpper(source[:1]) + source[1:], // Capitalize first letter
		JobID:  "temp-job-id",                            // In real implementation, generate unique job ID
	}

	u.Templates.ImportProcessing.Execute(w, r, data)
}

// ProcessExport handles exporting user data
func (u Users) ProcessExport(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())

	logger.Info("export requested", "user_id", user.ID)

	// TODO: Implement actual export functionality
	// For now, just return an error indicating it's not implemented yet
	http.Error(w, "Export functionality coming soon", http.StatusNotImplemented)
}

// ImportStatus checks the status of an import job
func (u Users) ImportStatus(w http.ResponseWriter, r *http.Request) {
	logger := context.Logger(r.Context())
	user := context.User(r.Context())
	jobID := r.URL.Query().Get("job_id")

	logger.Info("import status check", "user_id", user.ID, "job_id", jobID)

	// For now, simulate different states based on job_id
	// In a real implementation, you would check actual job status from database/queue
	data := struct {
		JobID          string
		Source         string
		ImportComplete bool
		ImportFailed   bool
		ImportedCount  int
		ErrorMessage   string
	}{
		JobID:  jobID,
		Source: "Pocket",
	}

	// Simulate processing states for demo purposes
	if jobID == "temp-job-id" {
		// For demo, show still processing
		data.ImportComplete = false
		data.ImportFailed = false
	} else {
		// For other job IDs, simulate completion
		data.ImportComplete = true
		data.ImportedCount = 42 // Demo value
	}

	u.Templates.ImportStatus.Execute(w, r, data)
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
