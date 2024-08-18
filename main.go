package main

import (
	"fmt"
	"net/http"
)

func handlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "<h1>Welcome dude!</h1>")
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<h1>You can <a href="http://google.com">here</a> contact us here</h1>`)
}

func faqHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
	<h1>FAQ Page</h1>
	<ul>
	<li>
		<b>Is there a free version?</b>
		Yes! We offer a free trial for 30 days on any paid plans.
	</li>
	<li>
		<b>What are your support hours?</b>
		We have support staff answering emails 24/7, though response
		times may be a bit slower on weekends.
	</li>
	<li>
		<b>How do I contact support?</b>
		Email us - <a href="mailto:support@lenslocked.com">support@lenslocked.com</a>
	</li>
	</ul>
	`)
}

func pathHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/contact":
		contactHandler(w, r)
	case "/faq":
		faqHandler(w, r)
	case "/":
		handlerFunc(w, r)
	default:
		http.NotFound(w, r)
	}
}

type MyHandler struct{}

func (MyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pathHandler(w, r)
}

func main() {
	fmt.Println("Starting server on port 8000")
	// http.ListenAndServe("localhost:8000", http.HandlerFunc(pathHandler))
	var handler MyHandler
	http.ListenAndServe("localhost:8000", handler)
}
