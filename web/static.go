package web

import (
	"html/template"
	"net/http"
)

func StaticHandler(tpl Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, r, nil)
	}
}

func FAQ(tpl Template) http.HandlerFunc {
	questions := []struct {
		Question string
		Answer   template.HTML
	}{
		{
			Question: "Is there a free version?",
			Answer:   "You can start using Pensieve for free. With premium, you will have higher limits, specially on AI features.",
		},
		{
			Question: "What are your support hours?",
			Answer:   "You can always contact me by sending email. Response times may be a bit slower on weekends.",
		},
		{
			Question: "How do I contact you?",
			Answer:   `Email me - <a href="mailto:arashThr@duck.com">arashThr@duck.com</a>`,
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tpl.Execute(w, r, questions)
	}
}
