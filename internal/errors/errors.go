package errors

import "errors"

var (
	ErrNotFound   = errors.New("models: resource could not be found")
	ErrEmailTaken = errors.New("models: email address is already in use")
	ErrInvalidUrl = errors.New("controller: url is invalid")

	// Stripe
	ErrNoStripeCustomer = errors.New("models: stripe customer not found")

	// Rate limiting
	ErrDailyLimitExceeded = errors.New("models: daily bookmark limit exceeded")
)
