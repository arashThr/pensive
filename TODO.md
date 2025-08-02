## Tech
- Check Cloudflare [D1](https://developers.cloudflare.com/d1/) for database
- Get `recollectra.com` domain

## Tasks

## Fixes
- [ ] getAllBookmarksForUser
- [ ] Getting non-existant bookmark: Something went wrong
- [ ] Faster build
- [ ] Send email for reset password takes time. Do we wait for sending email?
- [ ] Why YouTube is not working?
- [ ] failed to collect row: bookmark by link: too many rows in result set
    - When entering old link
- [x] Fix Telegram bot link parsing issue
- [x] Does not check for duplicates when importing links from pocket
- [x] Adding bookmark from website brings the image, but the extension does not:
    - https://world.hey.com/dhh/linux-crosses-magic-market-share-threshold-in-us-1f914771
- [x] Extension: Uncaught TypeError: Failed to construct 'URL': Invalid base URL
    - URLs without https
- [x] Limit size of Excerpt:
    - http://www.defmacro.org/ramblings/fp.html is the whole article
- [x] http and https are considered separate: http://en.wikipedia.org/wiki/Anthropic_principle
- [x] Use template for export to avoid duplicatin it in tab and export page
- [x] We're stuck in index page when the page does not have a title:
    -  https://apps.arashtaher.com/home 
    - Check if endpoint is valid
- [x] Remove Umami from edit page
- [x] Fix import: Links are imported multiple times
    - [x] Enable the import
- [x] `resetUrl := "http://localhost:8000/reset-password?" + values.Encode()`
- [x] Fix API token key SVG and the button alignment
- [x] Fix setting user premium after success: Listener had to be running
- [x] `Invalid request body: parsing time "2025-01-29" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T"` 
    - https://www.granola.ai/blog/dont-animate-height
- [x] Verification failure when we return to sing in/up after failed attempt
- [x] Error checking bookmark status: Don't status check for non-http websites
- [x] Update token in extensions
    - [x] Remove token when endpoint is changed
    - [x] Ping to make sure the API token in extension is valid (is it deleted?)
- [x] Move ai_content to content table
- [x] failed to get page: failed to perform request: Get "www.defmacro.org/2006/06/19/fp.html": unsupported protocol scheme ""
- [x] Fix redirects: http://www.defmacro.org/ramblings/fp.html
- [x] Fix the join problem with path: `https://apps.arashtaher.com//extension/auth`
- [x] Flex for the table

## Features
- [ ] Ping server from Telegram bot to check the validity of the token
- [ ] Limitation on the tokens for LLM
    - [ ] Accept user's API key
- [ ] Feedback link
- [ ] Use Pupeteer and playwright for getting the pages
- [ ] For Telegram/URL-only ingestion:
	• Maintain domain whitelist/blacklist
	• Or do HEAD request and reject:
	• Non-200
	• Login redirects
	• Cloudflare pages (__cf_bm cookies, JS challenges)
- [ ] Landing page: Here's how it works demo:
    - Save the link, show the summary
- Youtube
    - Option: Use [RapiAPI](https://rapidapi.com/solid-api-solid-api-default/api/youtube-transcript3/pricing)
    - `youtube-transcript-api`
- [ ] Premium: Get YouTube
- [ ] Observability
    - [ ] Add metrics
    - [ ] Send logs when extension crashes
    - [ ] Telegram logging
    - [ ] Remove time from logs
    - [ ] Zap is possible?
- [ ] Add like and dislike buttons
- [ ] Tags
    - [ ] Read tags from Pocket
- [ ] Short key for calling the extension
- [ ] Send verification email
    - [ ] Reminder to install Telegram and Extensions
- [ ] Create page for "Missing authorization code"
- [x] Google and GitHub auth
- [x] Add Firefox extenions
- [x] Back up db
    - [x] Create backups
    - [x] Preserve db backups on a remote server
    - [x] Automate db backup
- [x] Limitations configs (Free vs. Premium):
    - [x] Bookmarks
    - [x] Test limits
    - [x] AI and YouTube
- [x] Add: Warn me about personal pages
- [x] Add analytics
- [x] Upload the extensions
    - [x] Create the extension logo for Chrome store
- [x] Setup hooks for payment in production
- [x] Export links
- [x] Get excerpt and summaries from Genimi
- [x] Limit search result to 10 - Search better!
- [x] Use Gemeni
    - [x] Getting page text - [Inspiration](https://simedw.com/2025/06/23/introducing-spegel/)
    - [x] Generate markdown page for future
- [x] Reddit and Stackoverflow problem: Send the page text/HTML from the extension?
- [x] Send the HTML from exntension
    - [x] Clean up the HTML in extension
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
- [ ] Remove password - Use email validation
- [ ] readability.Check for links that don't have HTML
- [ ] Use temp dir for pocket imports
- [ ] If API is not set, or incorrect, don't send the request
- [ ] Add validation to create bookmark inputs
- [ ] Send payments to Amazon Eventbridge
- [ ] Add ping call from extension to an authorized endpoint
- [ ] Apply rate limit on requests
- [ ] Don't let user to create API token, or only one
    - [ ] Show the orginator of the token and date
    - [ ] Updating extension/bot should overwrite previous token
- [ ] Optional host permission + explicit user permission
- [ ] Add verified to user table
    - [ ] Verify email
- [x] Extension
    - Check auth endpoint
    - Delete token when siging out    
- [x] Move markdown from menu to the edit page as a button
    - Load it in the same page. It's good enough for now
- [x] Add recaptcha
- [x] Go to search page after signup
- [x] Rename files and folders: To much work for no reason
    - integrations to extension
    - models and services
- [x] Update DB: This would be a breaking change
    - [x] Separate tables: No need
    - [x] Rename tables
    - [x] Create the exports
    - [x] Update prod
        - Cheatsheet for all the required commands for working with docker
    - [x] Clean the local db
    - [x] Update FAQ
    - [x] Separate the tables: Cancelled
    - [x] Update the compose file
        - [x] Update postgres version
    - [x] Rename all the pensieve to pensive
    - [x] Unstash changes to get the rename of bookmark id to id
    - [x] create new prod db
    - [x] Caddyfile: getpensive.com
- [x] Separate tables
    - [x] Remove user id from bookmark table: No need
- [x] Cancelled: Move prod to Supabase or Cloudflare
    - Too little storage, too much cost. Instead, make the current solution better: backups and extending
- [x] Use readability on the client side in extension
- [x] Move tokens from sync to local
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
- [ ] Bookmark only the selected text
- [ ] Separate text extraction service
- [ ] Extension: Future search
- [ ] Google login
- [ ] Twitter posts
- [ ] Look for changes in bookmarks parent folder
- [ ] Tags from browser
- [ ] Slash command for bookmarks in Telegram
- [ ] Blue/green deployment
- [ ] Accept Postgres connection strings so people can own their data

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

## Integrations

### YouTube
- `ytplayer.config.args.raw_player_response.captions.playerCaptionsTracklistRenderer.captionTracks[0].languageCode`
- `yt-dlp --convert-subs=srt  --write-auto-sub --write-sub --sub-lang "en,en-us,en-GB,automatic-caption-en" --skip-download "<Link>"`

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
