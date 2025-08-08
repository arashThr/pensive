package service

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/arashthr/go-course/internal/auth/context/loggercontext"
	"github.com/arashthr/go-course/internal/auth/context/usercontext"
	"github.com/arashthr/go-course/internal/models"
)

type User struct {
	BookmarkModel    *models.BookmarkModel
	AuthTokenService *models.AuthTokenService
	EmailService     *EmailService
	Domain           string
}

// CurrentUserAPI handles the current user's information.
// @Produce json
// @Success 200 {object} struct{Email string; IsSubscribed bool; Tokens []models.ApiToken}
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Something went wrong"
// @Router /v1/api/user/me [get]
func (a *User) CurrentUserAPI(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())
	var data struct {
		Email              string
		IsSubscribed       bool
		EmailVerified      bool
		RemainingBookmarks int
	}
	data.Email = user.Email
	data.EmailVerified = user.EmailVerified
	data.IsSubscribed = user.IsSubscriptionPremium()

	// Get remaining bookmarks for unverified users
	remaining, err := a.BookmarkModel.GetRemainingBookmarks(user)
	if err != nil {
		logger.Warnw("failed to get remaining bookmarks", "error", err)
		data.RemainingBookmarks = 0
	} else {
		data.RemainingBookmarks = remaining
	}

	err = writeResponse(w, data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

// RequestVerificationEmailAPI handles sending verification emails for unverified users
// @Accept json
// @Produce json
// @Success 200 {object} struct{message string} "Verification email sent"
// @Failure 400 {object} ErrorResponse "Email already verified or other error"
// @Failure 500 {object} ErrorResponse "Failed to send email"
// @Router /v1/api/user/request-verification [post]
func (a *User) RequestVerificationEmailAPI(w http.ResponseWriter, r *http.Request) {
	user := usercontext.User(r.Context())
	logger := loggercontext.Logger(r.Context())

	if user.EmailVerified {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "ALREADY_VERIFIED",
			Message: "Email is already verified",
		})
		return
	}

	// Only allow password users to request verification (OAuth/passwordless users are auto-verified)
	if user.PasswordHash == nil {
		writeErrorResponse(w, http.StatusBadRequest, ErrorResponse{
			Code:    "NOT_APPLICABLE",
			Message: "Email verification not applicable for this account type",
		})
		return
	}

	err := a.sendEmailVerification(user.Email)
	if err != nil {
		logger.Errorw("request verification email via API", "error", err, "user_id", user.ID)
		writeErrorResponse(w, http.StatusInternalServerError, ErrorResponse{
			Code:    "EMAIL_SEND_FAILED",
			Message: "Failed to send verification email",
		})
		return
	}

	logger.Infow("verification email requested via API", "user_id", user.ID)
	var data struct {
		Message string `json:"message"`
	}
	data.Message = "Verification email sent"

	err = writeResponse(w, data)
	if err != nil {
		logger.Errorw("write response", "error", err)
	}
}

func (a *User) sendEmailVerification(email string) error {
	authToken, err := a.AuthTokenService.Create(email, models.AuthTokenTypeEmailVerification)
	if err != nil {
		return fmt.Errorf("create auth token for email verification: %w", err)
	}

	values := url.Values{
		"token": {authToken.Token},
	}
	verificationURL := fmt.Sprintf("%s/auth/verify-email?", a.Domain) + values.Encode()

	err = a.EmailService.EmailVerification(email, verificationURL)
	if err != nil {
		return fmt.Errorf("send email verification: %w", err)
	}

	return nil
}
