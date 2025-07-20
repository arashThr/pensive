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
			Question: "What is Pensive?",
			Answer:   "Pensive is a tool that helps you save and organize your bookmarks. It is a browser extension that allows you to save articles to your Pensive account.",
		},
		{
			Question: "Why you named it Pensive?",
			Answer:   "The name was actually supposed to be <a href='https://en.wikipedia.org/wiki/Pensieve'>Pensieve</a>, but I wasn't sure if I have the right to use it, also I didn't find a proper domain for it.",
		},
		{
			Question: "Is there a free version?",
			Answer:   "You can start using Pensive for free. With premium, you will have higher limits, specially on AI features.",
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
