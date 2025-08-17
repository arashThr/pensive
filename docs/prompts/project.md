# Project

## Definition
Build your own searchable memory of the web
Everything you read, hear, and watch should be indexed and searchable
Pensive is a web application that helps you seamlessly capture content without disrupting your workflow. To do so, it integrates with your browser and messaging apps.
It's kind of a bookmarking application, but it's also aware of the bookmark context.
The initial idea for the name of the app comes from Pensieve from the Harry Potter books. Just like in the book that you could preserve your precious memories, in here you can have a collection of the knowledge you already gained.

## Prompt

In our conversations, consider the following context: I am developing a web service that helps users to index and preserve everything they read online. There are different ways for users to specify a page as something they've read and want to find later. One example, and the main way, is a browser extension: the User can mark the pages they read to be indexed by clicking on the extension icon. Another way is to send the link to the Telegram bot of the application.

When we receive the link in our backend service, we will fetch the full page content in a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they've saved.
In the case of the extension, we send the cleaned version of the page HTML and metadata of the page to avoid scraping issues with sites like Reddit and Stack Overflow.

When the knowledge is captured, users can search for content of what they have saved.
I'm working on expanding the features: Like the option to send weekly/monthly summary of the highlights of your readings.

Import bookmarks option from Pocket is enabled. Mozilla shutting down Pocket was one of the main motivations for this project.

We also provide premium features which mostly use LLMs for getting better content: We use Google Gemini Flash lite to get summary and tags. Also, using LLMs, we will generate a clean markdown version of the page.
Premium users can save more content and their limitation for saves per day is higher.

Premium features will be extended in the future, including the option to support text extraction from YouTube videos for searchability.

## Technical specifications

The application is written in Go. It uses Postgres for storing data and for creating full text search index.
Webpages are created using HTML templates in Go, and we use HTMX for interactivity. Tailwind is used for styling.

### Authentication
The application supports multiple authentication methods:
- **OAuth**: GitHub and Google OAuth for quick signup/signin (automatically verified)
- **Passwordless Authentication**: Magic link authentication via email (automatically verified)
- **Traditional**: Email/password authentication with email verification required
- **API Token Authentication**: For browser extensions and telegram bot integration

Authentication is handled with secure session management, CSRF protection, and Cloudflare Turnstile for bot protection.

#### User Account Types & Limitations
- **OAuth users**: Automatically verified, full access to daily limits
- **Passwordless users**: Automatically verified, full access to daily limits  
- **Password users**: Email verification required, limited to 100 total bookmarks until verified
- **Verified users**: Daily bookmark limits (10 free, 100 premium)
- **Unverified users**: Total lifetime limit of 100 bookmarks to encourage verification
