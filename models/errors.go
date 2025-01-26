package models

import "errors"

var (
	ErrNotFound   = errors.New("models: resource could not be found")
	ErrEmailTaken = errors.New("models: email address is already in use")
	ErrInvalidUrl = errors.New("models: url is invalid")
)
