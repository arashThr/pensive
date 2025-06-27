## Tech
- Check Cloudflare [D1](https://developers.cloudflare.com/d1/) for database
- Get `recollectra.com` domain

## Fixes
- [ ] Fix the table
    - [ ] Flex for the table
    - [ ] Buttons above
- [ ] Why YouTube is not working?
- [ ] Ping to make sure the API token in extension is valid (is it deleted?)
- [ ] Remove token when endpoint is changed
    - Check if endpoint is valid
- [ ] Fix the join problem with path: `https://apps.arashtaher.com//extension/auth`
- [ ] Fix the message pop up in the extension

## Features
- [ ] Slash command for bookmarks in Telegram
- [ ] Ping server from Telegram bot to check the validity of the token
- [ ] Get token for extension without copy paste by redirect from website
    - Telegram bot
- [ ] Feedback link
- [ ] Google login
- [ ] Multiple sessions for the user
- [ ] Disconnect the extension
- [ ] Upload the extensions
- [ ] Show the raw content
- [ ] Separate text extraction service
- [x] Deep link auth for telegram (check Proposals)
- [x] Multi-page for bookmarks index page
- [x] Save the token for chat
- [x] Telegram bot: Add, delete
    - [x] Pretty config
    - [x] Remove the bookmark
- [x] Bookmarks
- [x] Delete bookmark with extension
        - [x] Site
        - [x] API
    - [x] Bookmarks
- [x] Pagination
- [x] Add search to API
    - Firefox does not support `service_worker`
- [x] Test base extension on Firefox
    - [x] Add tsvector
- [x] Bookmark search in database
- [x] Generate API  tokens

## Changes
- [ ] "Failed to parse image URL"
- [ ] Email only users - No password
- [ ] Keep the token name
- [ ] Clear the search results when text box is cleared
- [ ] Remove password and just use email
- [ ] Keep the token, and not the token hash
- [ ] Apply rate limit on requests
- [ ] Does user really need API key?
- [ ] If API is not set, or incorrect, don't send the request
- [ ] Move prod to Supabase
- [ ] Hid API Tokens and Subscriptions. Redirect to bookmarks
- [ ] Remove password - Email validation
- [ ] Multiple sessions
- [x] Rename api in model and service to avoid confusion
- [x] Replace multi-empty lines and spaces with one
- [x] Limit the number of tokens
- [x] Support multiple API tokens: Show them once and keep the hashed version
- [x] Trim excerpt for bookmark page
- [x] Add excerpt to bookmarks_content table
- [x] Separate table for content and ts_vector content to make it easier to back up data
- [x] Nano UUID for bookmarks
    - I have created a type to represent them
    - uint ids are good enough for my user case
- [x] Ids from uint to string

### Future
- [ ] Twitter posts
- [ ] Look for changes in bookmarks parent folder
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
