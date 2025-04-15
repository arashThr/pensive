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
- [ ] Delete bookmark with extension
- [ ] Telegram bot: Using API
- [ ] Multiple sessions for the user
- [ ] Get token for extension without copy paste by redirect from website
- [ ] Bookmarks
    - [ ] Look for changes in bookmarks parent folder
    - [ ] Remove the bookmark
    - [ ] Pretty config
- [ ] Twitter posts
- [ ] Multi-page for bookmarks index page

## Changes
- [x] Ids from uint to string
    - uint ids are good enough for my user case
    - I have created a type to represent them
- [x] Nano UUID for bookmarks
- [x] Separate table for content and ts_vector content to make it easier to back up data
- [x] Add excerpt to bookmarks_content table
- [x] Trim excerpt for bookmark page
- [x] Support multiple API tokens: Show them once and keep the hashed version
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
