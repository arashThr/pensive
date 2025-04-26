## Tech
- Check Cloudflare [D1](https://developers.cloudflare.com/d1/) for database
- Get `recollectra.com` domain

## Features
- [x] Generate API  tokens
- [x] Bookmark search in database
    - [x] Add tsvector
- [x] Test base extension on Firefox
    - Firefox does not support `service_worker`
- [x] Add search to API
- [x] Pagination
    - [x] Bookmarks
        - [x] API
        - [x] Site
- [x] Delete bookmark with extension
- [x] Bookmarks
    - [x] Remove the bookmark
    - [x] Pretty config
- [x] Telegram bot: Add, delete
- [ ] Save the token for chat
- [ ] Respond with duplicate with link exists
- [ ] Multiple sessions for the user
- [ ] Get token for extension without copy paste by redirect from website
- [ ] Twitter posts
- [ ] Look for changes in bookmarks parent folder
- [ ] Multi-page for bookmarks index page
- [ ] Deep link auth for telegram (check Proposals)
- [ ] Ping server from Telegram bot to check the validity of the token

## Changes
- [x] Ids from uint to string
    - uint ids are good enough for my user case
    - I have created a type to represent them
- [x] Nano UUID for bookmarks
- [x] Separate table for content and ts_vector content to make it easier to back up data
- [x] Add excerpt to bookmarks_content table
- [x] Trim excerpt for bookmark page
- [x] Support multiple API tokens: Show them once and keep the hashed version
- [x] Limit the number of tokens
- [ ] Remove password and just use email
- [ ] Clear the search results when text box is cleared
- [ ] Keep the token name
- [ ] API token page instead of fetching it
- [ ] Keep the token, and not the token hash
- [ ] Email only users - No password
- [ ] Replace multi-empty lines and spaces with one
- [ ] "Failed to parse image URL"
- [ ] Tags from browser

### Stripe
- [x] change `IsSubscribed` to subStatus
    - think about `past_due`
- [x] add `cancel_at_period_end` to subscription table
    - listen to event updates
    - update current periods when invoice when you receive a customer update: it will also cover the cancelation
- [ ] Idempotency in stripe webhooks
- [ ] correct type for stripe amount in postgres
- [ ] update cuurent period end time after invoice paid
- [ ] use `4000 0000 0000 0341` to simulate payement failed

## Fix
- [x] Fix the Caddy file issue
- [x] Don't fetch duplicates
- [x] Fix the HTTP issue on production 
    - tls: failed to verify certificate: x509: certificate signed by unknown authority: Scratch did not have ce-certificates
- [ ] Remove telebot: Thousands of dependecies

# Proposals

## Telegram Auth

Deep Link from Website to Telegram with Token Exchange

#### Flow Overview
- User Action on Website: The user, already logged into the website, clicks a "Connect Telegram" button or link.
- Deep Link to Telegram: The website generates a unique deep link (e.g., https://t.me/YourBot?start=unique_state) and redirects the user to Telegram.
- Bot Receives State: The user opens the link in Telegram, which sends a /start unique_state command to the bot.
- Token Exchange: The bot sends the unique_state to the API, which verifies it, associates the Telegram user ID with the authenticated website user, and issues a Bearer token.
- Bot Stores Token: The bot stores the token and uses it for future API requests on behalf of the user.
- User Confirmation: The bot confirms successful integration to the user in Telegram.
