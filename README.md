
## Run 

### Locally

- `docker compose up` to start the database and other services
- Run `modd` to start the server in watch mode
- Telegram bot: `go run integrations/telegram/bot.go`

Or you can run all the services locally with docker compose:
`docker compose -f compose.yml -f compose.local.yml up`

### Production

Check `scripts/post-receive-hook`

## Database
__Connect to db__:
- `docker compose exec -it db psql -U postgres -d pensieve`

Go to `base-app` tag for the basic setup of a website with auth

Run migrations locally:
`migrate -path db/migrations -database 'postgres://postgres:postgres@localhost:5432/pensieve?sslmode=disable' up 1`

### Production

Run `psql` in the container: `docker exec -it go-web-db-1 bash`

### Stripe

Run these for testing the payments locally:

- Listen to events: `stripe listen`
- Forward webhooks to local: `stripe listen --forward-to localhost:8000/api/stripe-webhooks`
- Trigger a payment: `stripe trigger payment_intent.succeeded`
