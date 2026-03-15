# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go web application for bookmarking and content management, featuring:
- Web scraping and content extraction using Mozilla Readability
- AI-powered content summarization with Google Gemini
- Semantic search using pgvector and embeddings (gemini-embedding-001)
- RAG-based Q&A system for asking questions about bookmarked content
- Multiple authentication methods (GitHub, Google, Telegram OAuth, password-based)
- Email verification system for password signups with usage limitations
- Browser extensions (Chrome/Firefox) for easy bookmark saving
- Telegram bot integration
- Stripe payment processing for premium features
- PostgreSQL database with pgvector extension and migrations
- Docker containerization
- **AI-generated audio podcasts** via Google Cloud TTS (Gemini 2.5 Flash TTS), delivered by email and/or Telegram
  - Weekly summary: configurable delivery day, email + Telegram delivery
  - Daily briefing: configurable hour + timezone, Telegram-only delivery
  - Both schedules share a single `podcast_schedules` table with a `schedule_type` column (`weekly` / `daily`)
  - Long scripts are split into ≤3500-byte chunks and concatenated with `ffmpeg`

## Development Commands

### Local Development
- `modd` - Start the server in watch mode (auto-rebuild on file changes)
- `go run cmd/server/main.go` - Run the main server directly
- `docker compose up` - Start PostgreSQL and other services
- `docker compose -f compose.yml -f compose.local.yml up` - Run all services locally with Docker

### Database Management
- `migrate -path db/migrations -database 'postgres://postgres:postgres@localhost:5432/pensive?sslmode=disable' up 1` - Run a single migration
- Run `scripts/init-db.sh` to initialize the database

### Building and Testing
- `go build -o server_binary ./cmd/server/` - Build the server binary
- `make run` - Use Makefile to run the application
- No test files found in this codebase

### Additional Services
- Telegram bot: `go run integrations/telegram/bot.go`
- `go run cmd/telegram/main.go` - Alternative telegram entry point

### Podcast / TTS Testing (Local)
- Admin trigger endpoint: `POST /admin/podcast/trigger` — manually fires a weekly episode for a user
- `GCP_PROJECT_ID` and `GCP_SERVICE_ACCOUNT_PATH` must be set; ADC is used locally if `GCP_SERVICE_ACCOUNT_PATH` is empty
- `ffmpeg` must be installed locally for multi-chunk audio concatenation

### Stripe Testing (Local)
- `stripe listen` - Listen to Stripe events
- `stripe listen --forward-to localhost:8000/api/stripe-webhooks` - Forward webhooks to local server
- `stripe trigger payment_intent.succeeded` - Trigger test payment

## Architecture

### Core Structure
- `cmd/` - Entry points for different services (server, telegram, experiments)
- `internal/` - Private application code:
  - `auth/` - Authentication logic (OAuth, sessions, users)
  - `config/` - Configuration management
  - `db/` - Database connection and migrations
  - `models/` - Data models (User, Bookmark, Session, etc.)
  - `service/` - Business logic controllers
  - `types/` - Custom type definitions
  - `validations/` - Input validation utilities
- `web/` - Frontend templates and static assets
- `integrations/` - Browser extensions and telegram bot

### Key Technologies
- **Framework**: Chi router for HTTP routing
- **Database**: PostgreSQL with pgx driver
- **Templating**: Go templates with HTMX for dynamic content
- **Auth**: Multiple OAuth providers + session-based auth + password-based auth with email verification
- **AI**: Google Gemini for content summarization and podcast script generation
- **TTS**: Google Cloud TTS (Gemini 2.5 Flash TTS) via HTTP API with oauth2 ADC / service account auth
- **Audio**: `ffmpeg` (in Alpine runtime image) for concatenating OGG Opus chunks
- **Payments**: Stripe for subscription management
- **Logging**: Zap for structured logging
- **CSS**: TailwindCSS (built in separate container)

### Semantic Search & RAG Features
- **pgvector**: Vector similarity search using gemini-embedding-001 (768 dimensions) with HNSW index
- **Dual Search Modes**:
  - **Search Tab**: Traditional full-text search for instant results
  - **Ask AI Tab**: RAG-based Q&A using vector search + Gemini to answer questions about bookmarks
- **Implementation**: `AskQuestion()` in `internal/models/bookmark.go` retrieves relevant bookmarks via vector search, then generates AI answers with source citations
- **UI**: Tabbed HTMX interface with loading indicators and error handling for AI service failures

### Page Structure
- **Home Page** (`/home`): Main interface with two tabs:
  - Search tab: Full-text search and paginated bookmark list (default view)
  - Ask AI tab: RAG-based Q&A about bookmarked content
- **Individual Bookmark Pages** (`/bookmarks/{id}`): View and edit bookmarks
- **Settings Page** (`/users/me`): User profile, tokens, import/export, data management
  - Preferences tab: weekly podcast (day, email/Telegram delivery) + daily briefing (hour, timezone, Telegram-only)

### Podcast Feature
- **Script generation**: Gemini generates a full markdown podcast script from recent bookmarks, then a second Gemini call strips it to plain narration text
- **TTS**: `callGoogleTTS` in `internal/service/podcast.go` splits the script into ≤3500-byte chunks (paragraph → sentence boundaries), calls Google Cloud TTS sequentially per chunk, writes temp OGG files, and concatenates with `ffmpeg -f concat -c copy`
- **Schedulers**: Two background goroutines — `StartScheduler` (weekly) and `StartDailyScheduler` (daily) — each tick every minute and call `GetDue(scheduleType)` on the unified `podcast_schedules` table
- **State machine**: `pending → processing → sent/failed/timed_out`; `ReapTimedOut` reclaims stale processing rows across all types
- **Delivery**: Completed audio is emailed as attachment and/or sent via Telegram bot depending on user preferences
- **Admin trigger**: `POST /admin/podcast/trigger` fires a weekly episode immediately for a given user (internal endpoint, not user-facing)

### Configuration
- Environment variables loaded via `godotenv`
- Configuration centralized in `internal/config/config.go`
- Uses PostgreSQL connection pooling
- CSRF protection with gorilla/csrf

### Database Schema
- Users, sessions, and password reset tables
- Email verification system with auth tokens
- API tokens for extension/bot authentication
- Library/bookmarks with full-text search (tsvector)
- Vector embeddings (vector(768)) with HNSW index for semantic search
- Stripe subscription management tables
- Import job tracking for Pocket imports
- Telegram authentication tables
- `summaries_pref` — per-user podcast preferences (enabled, day, email, telegram, daily_enabled, daily_hour, daily_timezone)
- `podcast_schedules` — unified schedule table for both weekly and daily episodes; `schedule_type` column (`weekly`/`daily`); UNIQUE on `(user_id, schedule_type)`; uses `podcast_schedule_status` enum

### User Account System
- **OAuth users** (GitHub, Google, Telegram): Automatically verified, full access
- **Passwordless users**: Magic link authentication, automatically verified, full access  
- **Password users**: Email verification required, limited to 100 total bookmarks until verified
- **Verified users**: Daily limits (10 free, 100 premium)

### Deployment
- Docker multi-stage build (Tailwind → Go build → Alpine runtime)
- Alpine runtime image includes `ffmpeg` (`apk add --no-cache ffmpeg`) for podcast audio concatenation
- `.env` is **not** baked into the image; it is mounted at runtime from `~/credentials/.env` on the server
- GCP service account JSON is mounted from `~/credentials/service-account.json` into the container at the path defined by `GCP_SERVICE_ACCOUNT_PATH`
- Production deployment via `post-receive-hook` script with `--env-file ~/credentials/.env`
- Supports environment-specific compose files
- Database migrations handled during startup

## Important Files
- `modd.conf` - File watcher configuration for development
- `go.mod` - Go module dependencies
- `compose.yml` - Docker services configuration
- `compose.production.yml` - Production overrides (volume mounts for credentials, resource limits)
- `internal/db/migrations/` - Database schema migrations
- `internal/models/podcast.go` - `PodcastScheduleRepo` — unified weekly+daily schedule repo
- `internal/service/podcast.go` - Podcast generation, TTS chunking, scheduler goroutines
- `web/templates/user/preferences-tab.gohtml` - Weekly + daily podcast preference UI
- `web/templates/` - HTML templates
- `Dockerfile` - Multi-stage container build
- `credentials/google.md` - Notes on GCP service account setup (file itself is gitignored)