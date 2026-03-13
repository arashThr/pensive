package auth

import (
	"fmt"
	"net/http"
	"strings"
)

type AdminMiddleware struct {
	user     string
	password string
}

func NewAdminMw(u string, p string) *AdminMiddleware {
	return &AdminMiddleware{
		user:     u,
		password: p,
	}
}

func (amw *AdminMiddleware) AuthAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		fmt.Println("Admin auth attempt:", user)
		fmt.Println("Admin auth pass:", pass)
		fmt.Println("Admin auth success:", ok, user, amw.user, pass, amw.password)
		if !ok ||
			!strings.EqualFold(strings.TrimSpace(user), strings.TrimSpace(amw.user)) ||
			!strings.EqualFold(strings.TrimSpace(pass), strings.TrimSpace(amw.password)) {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
