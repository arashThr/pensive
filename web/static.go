package web

import (
	"html/template"
	"net/http"
)

func StaticHandler(title string, tpl Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Title string
		}
		data.Title = title
		tpl.Execute(w, r, data)
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
			Question: "Is Pensive ready for production use?",
			Answer:   `<strong>No, Pensive is currently in beta.</strong> This means the service is still being tested and improved. While we work hard to protect your data, there's a possibility that data could be lost during updates, migrations, or unexpected issues. We recommend regularly exporting your bookmarks and not relying on Pensive as your only storage method for critical information. See our <a href="/privacy">Privacy Policy</a> for more details.`,
		},
		{
			Question: "What does 'beta' mean for my data?",
			Answer:   "During the beta phase, your data may be lost or wiped due to system updates, database migrations, or technical issues. We'll provide advance notice when possible, but unexpected data loss can occur. We strongly recommend backing up important bookmarks and treating this as a testing environment rather than a permanent storage solution.",
		},
		{
			Question: "Why you named it Pensive?",
			Answer:   "The name was actually supposed to be <a href='https://en.wikipedia.org/wiki/Pensieve'>Pensieve</a>, but I wasn't sure if I have the right to use it, also I didn't find a proper domain for it.",
		},
		{
			Question: "Is there a free version?",
			Answer:   "You can start using Pensive for free with AI features included. With premium, you get higher daily limits (100 vs 10 bookmarks/day)",
		},
		{
			Question: "Do you have a mobile app?",
			Answer:   "No, but you don't need to install a new app to work with Pensive. Just connect you account to Pensive Telegram bot to save articles and search in your library.",
		},
		{
			Question: "What language is Pensive available in?",
			Answer:   "At this point, Pensive is only available in English and there are no plans to support other languages.",
		},
		{
			Question: "How can I backup my bookmarks?",
			Answer:   "You can export your bookmarks from your account settings page. We recommend doing this regularly during the beta phase to protect against potential data loss. The export feature allows you to download your saved content in standard formats.",
		},
		{
			Question: "What are your support hours?",
			Answer:   "You can always contact me by sending email. Response times may be a bit slower on weekends.",
		},
		{
			Question: "How do I contact you?",
			Answer:   `Email me - <a href="mailto:arash.thr@live.com">arash.thr@live.com</a>`,
		},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			Title     string
			Questions []struct {
				Question string
				Answer   template.HTML
			}
		}
		data.Title = "Frequently Asked Questions"
		data.Questions = questions
		tpl.Execute(w, r, data)
	}
}
