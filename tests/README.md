# Tests

Just a series of resources that could be used for manual testing.

## How to create new accounts

### Web

Go to `http://localhost:8000/home` and sign up.
To verify the email, go to Mailtrap Sandbox https://mailtrap.io/inboxes/
Run Strip CLI and listen for event: `stripe listen --forward-to localhost:8000/api/stripe-webhooks`
Use `4242 4242 4242 4242` as the card number for buying premium.

### Telegram

If you want to enable Telegram, you have to have it running:
`docker compose -f compose.yml compose.local.yml up`

### Install the extension

In Chrome, go to `chrome://extensions/`
Click "Load unpacked" and select the Chrome folder in Integrations directory
