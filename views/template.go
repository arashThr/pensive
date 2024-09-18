package views

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/arashthr/go-course/templates"
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

func ParseTemplate(filePath string) (Template, error) {
	template, err := template.ParseFS(templates.FS, filePath)
	if err != nil {
		return Template{}, fmt.Errorf("parse fs template: %w", err)
	}
	return Template{
		htmlTemplate: template,
	}, nil
}

func (t Template) Execute(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := t.htmlTemplate.Execute(w, data)
	if err != nil {
		log.Printf("executing template failed: %v", err)
		http.Error(w, "There was an error executing the template", http.StatusInternalServerError)
	}
}
