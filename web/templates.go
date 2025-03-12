package web

import "net/http"

type NavbarMessage struct {
	Message string
	IsError bool
}

type Template interface {
	Execute(w http.ResponseWriter, r *http.Request, data any, err ...NavbarMessage)
}
