# Pensive

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://www.docker.com/)

**Your searchable memory of the web** - An open-source knowledge management tool that captures full article content and makes everything searchable.

Pensive helps developers, researchers, and knowledge workers build their personal library of web content with full-text search, AI-powered summaries, and seamless saving from anywhere.

## ✨ Features

- **Full-text search** across all saved content
- **AI-powered summaries** and automatic tagging
- **Browser extensions** for Chrome and Firefox
- **Telegram bot** integration for mobile saving
- **Data portability** - import/export your bookmarks
- **Self-hostable** with Docker
- **Open source** under AGPL-3.0 license

## 🏗️ Components

This repository contains all components of the Pensive ecosystem:

| Component | Location | Description |
|-----------|----------|-------------|
| **Backend Server** | `cmd/server/` | Go web server with API and web UI |
| **Browser Extensions** | `integrations/chrome/`, `integrations/firefox/` | Chrome and Firefox extensions |
| **Telegram Bot** | `cmd/telegram/` | Telegram bot for mobile saving |
| **Shared Backend** | `internal/` | Shared Go libraries (auth, database, models) |
| **Web Frontend** | `web/` | HTML templates and static assets |

## 🚀 Quick Start

### Option 1: Try the Hosted Version
Visit [getpensive.com](https://getpensive.com) for a free trial with 10 saves per day.

### Option 2: Self-Host with Docker

```bash
# Clone the repository
git clone https://github.com/yourusername/pensive.git
cd pensive

# Start with Docker Compose
docker compose up
```

### Option 3: Local Development

**Prerequisites:**
- Go 1.24+
- PostgreSQL
- NPM (for building CSS file from Tailwind)

**Setup:**
```bash
# Start database and services
docker compose up

# Start the server in watch mode
modd

# Run Telegram bot (optional)
go run cmd/telegram/main.go
```

## 🛠️ Development

### Database Migrations
```bash
migrate -path db/migrations -database 'postgres://postgres:postgres@localhost:5432/pensive?sslmode=disable' up 1
```

### Browser Extensions
Build the Ready-to-Publish zip file for extensions by running this command:
```bash
cd integrations
./build.sh  # Builds both Chrome and Firefox extensions
```
Build process with use the latest git tag to version the extension.

### Stripe Testing (for premium features)
```bash
stripe listen --forward-to localhost:8000/api/stripe-webhooks
stripe trigger payment_intent.succeeded
```

## 🏢 Deployment

### Self-Hosted Production
```bash
docker compose -f compose.yml -f compose.production.yml up
```

### Managed Hosting
For managed hosting with higher limits, visit [getpensive.com](https://getpensive.com) - $5/month supports the project development.

## 🤝 Contributing

We welcome contributions! The project is structured to make it easy to contribute to specific components:

- **Backend**: Contribute to API, database, or core features
- **Extensions**: Improve browser integration and user experience  
- **Bot**: Enhance Telegram bot functionality
- **Frontend**: Work on web UI and user interface

## 📄 License

AGPL-3.0 License - see [LICENSE](LICENSE) file for details.

This project is licensed under the GNU Affero General Public License v3.0. This means you can use, modify, and distribute this software, but any modifications must be made available under the same license, including for web services.

## 🔗 Links

- **Website**: [getpensive.com](https://getpensive.com)
- **Chrome Extension**: [Chrome Web Store](https://chromewebstore.google.com/detail/pensive-save-search-what/klmginbbicjdpaodcbokdjbhnbaocomd)
- **Firefox Extension**: [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/pensive/)
- **Telegram Bot**: [@GetPensiveBot](https://t.me/GetPensiveBot)

---

**Built with ❤️ for developers and researchers who want to own their data.**