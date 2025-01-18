package views

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/arashthr/go-course/context"
	"github.com/arashthr/go-course/models"
	"github.com/arashthr/go-course/templates"
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
	tpl := template.New(filePaths[0])
	tpl.Funcs(template.FuncMap{
		"csrfField": func() (template.HTML, error) {
			return "", fmt.Errorf("csrfField not implemented")
		},
		"currentUser": func() (template.HTML, error) {
			return "", fmt.Errorf("current user not implemented")
		},
		"errors": func() []string {
			return nil
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

func (t Template) Execute(w http.ResponseWriter, r *http.Request, data any, errs ...error) {
	tpl, err := t.htmlTemplate.Clone()
	if err != nil {
		log.Printf("cloning template failed: %v", err)
		http.Error(w, "There was an error serving your request", http.StatusInternalServerError)
		return
	}
	tpl = tpl.Funcs(
		template.FuncMap{
			"csrfField": func() template.HTML {
				return csrf.TemplateField(r)
			},
			"currentUser": func() *models.User {
				return context.User(r.Context())
			},
			"errors": func() []string {
				var errMessage []string
				for _, err = range errs {
					errMessage = append(errMessage, err.Error())
				}
				return errMessage
			},
		},
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var buf bytes.Buffer
	err = tpl.Execute(&buf, data)
	if err != nil {
		log.Printf("executing template: %v", err)
		http.Error(w, "There was an error executing the template", http.StatusInternalServerError)
		return
	}
	io.Copy(w, &buf)
}
