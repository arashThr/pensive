package errors

import "errors"

// Sentinel errors for the auth module.
var (
	ErrNotFound   = errors.New("resource could not be found")
	ErrEmailTaken = errors.New("email address is already in use")
	ErrExpired    = errors.New("token has expired")
	ErrInvalidToken = errors.New("invalid token")
)

// Is and As delegate to the stdlib so callers can use this package directly.
var (
	Is = errors.Is
	As = errors.As
)

// Public wraps an error with a user-visible message.
func Public(err error, msg string) error {
	return &publicError{msg: msg, err: err}
}

type publicError struct {
	msg string
	err error
}

func (pe *publicError) Public() string  { return pe.msg }
func (pe *publicError) Error() string   { return pe.err.Error() }
func (pe *publicError) Unwrap() error   { return pe.err }
