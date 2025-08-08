package views

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/arashthr/go-course/internal/auth/context/loggercontext"
	"github.com/arashthr/go-course/internal/auth/context/usercontext"
	"github.com/arashthr/go-course/internal/models"
	"github.com/arashthr/go-course/web"
	"github.com/arashthr/go-course/web/templates"
	"github.com/gorilla/csrf"
)

type Template struct {
	htmlTemplate *template.Template
}

func Must(tpl Template, err error) Template {
	if err != nil {
		panic(err)
	}
	return tpl
}

func ParseTemplate(filePaths ...string) (Template, error) {
	tpl := template.New(path.Base(filePaths[0]))
	tpl.Funcs(template.FuncMap{
		"csrfField": func() (template.HTML, error) {
			return "", fmt.Errorf("csrfField not implemented")
		},
		"csrfToken": func() (string, error) {
			return "", fmt.Errorf("csrfToken not implemented")
		},
		"currentUser": func() (template.HTML, error) {
			return "", fmt.Errorf("current user not implemented")
		},
		"messages": func() []web.NavbarMessage {
			return nil
		},
		"safe": func(s string) template.HTML {
			return template.HTML(s) // Trust ts_headline output
		},
		"isProduction": func() (template.HTML, error) {
			return "", fmt.Errorf("isProduction not implemented")
		},
		"shouldUseAnalytics": func() (template.HTML, error) {
			return "", fmt.Errorf("shouldUseAnalytics not implemented")
		},
		"isSubscriptionEnabled": func() (template.HTML, error) {
			return "", fmt.Errorf("isSubscriptionEnabled not implemented")
		},
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
	})
	tpl, err := tpl.ParseFS(templates.FS, filePaths...)
	if err != nil {
		return Template{}, fmt.Errorf("parse fs template: %w", err)
	}
	return Template{
		htmlTemplate: tpl,
	}, nil
}

func (t Template) Execute(w http.ResponseWriter, r *http.Request, data any, navMsgs ...web.NavbarMessage) {
	logger := loggercontext.Logger(r.Context())
	tpl, err := t.htmlTemplate.Clone()
	if err != nil {
		logger.Errorw("cloning template failed", "error", err)
		http.Error(w, "There was an error serving your request", http.StatusInternalServerError)
		return
	}
	disableAnalytics := false
	requestPath := r.URL.Path
	re := regexp.MustCompile(`\/bookmarks/\w+\/edit`)
	if re.MatchString(requestPath) {
		disableAnalytics = true
	}

	tpl = tpl.Funcs(
		template.FuncMap{
			"csrfField": func() template.HTML {
				return csrf.TemplateField(r)
			},
			"csrfToken": func() string {
				return csrf.Token(r)
			},
			"currentUser": func() *models.User {
				return usercontext.User(r.Context())
			},
			"messages": func() []web.NavbarMessage {
				return navMsgs
			},
			"isProduction": func() bool {
				return os.Getenv("ENVIRONMENT") == "production"
			},
			"isSubscriptionEnabled": func() bool {
				return os.Getenv("SUBSCRIPTION_ENABLED") == "true"
			},
			"shouldUseAnalytics": func() bool {
				return !disableAnalytics
			},
		},
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf bytes.Buffer
	err = tpl.Execute(&buf, data)
	if err != nil {
		logger.Errorw("executing template", "error", err)
		http.Error(w, "There was an error executing the template", http.StatusInternalServerError)
		return
	}
	io.Copy(w, &buf)
}
