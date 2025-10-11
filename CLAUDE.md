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
- **AI**: Google Gemini for content summarization
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

### User Account System
- **OAuth users** (GitHub, Google, Telegram): Automatically verified, full access
- **Passwordless users**: Magic link authentication, automatically verified, full access  
- **Password users**: Email verification required, limited to 100 total bookmarks until verified
- **Verified users**: Daily limits (10 free, 100 premium)

### Deployment
- Docker multi-stage build (Tailwind → Go build → Alpine runtime)
- Production deployment via `post-receive-hook` script
- Supports environment-specific compose files
- Database migrations handled during startup

## Important Files
- `modd.conf` - File watcher configuration for development
- `go.mod` - Go module dependencies
- `compose.yml` - Docker services configuration
- `internal/db/migrations/` - Database schema migrations
- `web/templates/` - HTML templates
- `Dockerfile` - Multi-stage container build