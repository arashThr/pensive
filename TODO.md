## Tech
- Check Cloudflare [D1](https://developers.cloudflare.com/d1/) for database
- Get `recollectra.com` domain

## Fixes
- [ ] Does not check for duplicates when importing links from pocket
- [ ] Extension: Uncaught TypeError: Failed to construct 'URL': Invalid base URL
    - URLs without https
- [ ] Limit size of Excerpt:
    - http://www.defmacro.org/ramblings/fp.html is the whole article
- [ ] Why YouTube is not working?
- [ ] http and https are considered separate: http://en.wikipedia.org/wiki/Anthropic_principle
- [ ] We're stuck in index page when the page does not have a title:
    -  https://apps.arashtaher.com/home 
- [ ] Remove token when endpoint is changed
    - Check if endpoint is valid
- [ ] failed to collect row: bookmark by link: too many rows in result set
    - When entering old link
- [ ] Avoid duplicates: https://en.wikipedia.org/wiki/Anthropic_principle
- [x] failed to get page: failed to perform request: Get "www.defmacro.org/2006/06/19/fp.html": unsupported protocol scheme ""
- [x] Fix redirects: http://www.defmacro.org/ramblings/fp.html
- [x] Fix the join problem with path: `https://apps.arashtaher.com//extension/auth`
- [x] Flex for the table

## Features
- [ ] Ping server from Telegram bot to check the validity of the token
- [ ] Google login
- [ ] Separate text extraction service
- [ ] Extension: Future search
- [ ] Add premium layer - After alpha release
- [ ] Export links
- [ ] Disconnect the extension
- [ ] Upload the extensions
- [ ] Captcha for Sign in
- [ ] Feedback link
- [ ] Use Pupeteer and playwright for getting the pages
- [ ] Use Gemeni
    - [ ] Getting page text - [Inspiration](https://simedw.com/2025/06/23/introducing-spegel/)
    - [ ] Generate markdown page for future
- [ ] Premium: Get YouTube
    - Option: Use [RapiAPI](https://rapidapi.com/solid-api-solid-api-default/api/youtube-transcript3/pricing)
    - `youtube-transcript-api`
- [ ] Limit search result to 10
- [x] Extension: Click to save
- [x] Parse Pocket export
- [x] Show the raw content
- [x] Multiple sessions for the user
- [x] Get token for extension without copy paste by redirect from website
    - Telegram bot
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
- [ ] Use temp dir for pocket imports
- [ ] Apply rate limit on requests
- [ ] Don't let user to create API token, or only one
    - [ ] Show the orginator of the token and date
    - [ ] Updating extension/bot should overwrite previous token
- [ ] Rename users_bookmarks table to bookmarks
- [ ] Remove password - Use email validation
- [ ] Use zap for logging
- [ ] If API is not set, or incorrect, don't send the request
- [ ] Move prod to Supabase or Cloudflare
- [x] Clear the search results when text box is cleared
- [x] Does user really need API key?
    - No
- [x] Remove token from extensions settings page
- [x] Hid API Tokens and Subscriptions. Redirect to bookmarks after login
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
- [ ] Ping to make sure the API token in extension is valid (is it deleted?)
- [ ] Slash command for bookmarks in Telegram

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
