package errors

import "errors"

var (
	ErrNotFound   = errors.New("resource could not be found")
	ErrEmailTaken = errors.New("email address is already in use")
	ErrInvalidUrl = errors.New("controller: url is invalid")

	// Stripe
	ErrNoStripeCustomer = errors.New("stripe customer not found")

	// Rate limiting
	ErrDailyLimitExceeded          = errors.New("daily bookmark limit exceeded")
	ErrUnverifiedUserLimitExceeded = errors.New("unverified user bookmark limit exceeded")
)
