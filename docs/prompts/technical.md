# Technical specifications

The application is written in Go. It uses Postgres for storing data and for creating full text search index.
Webpages are created using HTML templates in Go, and we use HTMX for interactivity. Tailwind is used for styling.

## Sign in/ Sign up

For sign up and sign in we have two methods:

- Traditional email/password authentication with bcrypt hashing
- OAuth2 integration with GitHub and Google (Authorization Code Flow)
Then the credentials will be save using session-based authentication with secure cookies.

We also have API token authentication for browser extensions and Telegram bot
