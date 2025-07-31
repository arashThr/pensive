
# Run 

## Locally

- `docker compose up` to start the database and other services
- Run `modd` to start the server in watch mode
- Telegram bot: `go run integrations/telegram/bot.go`

Or you can run all the services locally with docker compose:
`docker compose -f compose.yml -f compose.local.yml up`

### Stripe

Run these for testing the payments locally:

- Listen to events: `stripe listen`
- Forward webhooks to local: `stripe listen --forward-to localhost:8000/api/stripe-webhooks`
- Trigger a payment: `stripe trigger payment_intent.succeeded`

## Production

Deployment to production happens by executing the `post-receive-hook` in bare repo on the server.  
Check `scripts/post-receive-hook`

### Database

Run migrations locally:
`migrate -path db/migrations -database 'postgres://postgres:postgres@localhost:5432/pensive?sslmode=disable' up 1`

## Services
- Metrcis: [Umami](https://cloud.umami.is/settings/websites)
