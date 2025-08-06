# Project

## Define
Build your own searchable memory of the web
Everything you read, hear, and watch should be indexed and searchable
Pensive is a web application that helps you seamlessly capture content without disrupting your workflow. To do so, it integrates with your browser and messaging apps.
It's kind of a bookmarking application, but it's also aware of the bookmark context.
The name comes from Pensieve from the Harry Potter books. Just like in the book you could preserve your precious memories, in here you can have a collection of the knowledge you already gained.

## Prompt

In our conversations, consider the following context: I am developing a web service that helps users to index and preserve everything they read online. There are different ways for users to specify a page as something they've read and want to find later. One example, and the main way, is a browser extension: the User can mark the pages they read to be indexed by clicking on the extension icon. Another way is to send the link to the Telegram bot of the application.

When we receive the link in our backend service, we will fetch the full page content to a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they've saved.
In the case of the extension, we send the whole HTML and metadata of the page to avoid scraping issues with sites like Reddit and Stack Overflow (This has been paused for now, since I want to investigate the best solution: We don't want to send critical information from the logged in page).

When the knowledge is captured, users can search for content of what they have saved.
I'm working on expanding the features: Like the option to send weekly/monthly summary of the highlights of your readings, and importing bookmarks from Pocket.

We also provide premium features which mostly use LLMs for getting better content: We use Google Gemini Flash lite to get summary and tags. Also, using LLMs, we will generate a clean markdown version of the page.
Premium users can save more content and their limitation for saves per day is higher.

Premium features will be extended in the future, including the option to support text extraction from YouTube videos for searchability.

## Authentication
The application supports multiple authentication methods:
- **OAuth**: GitHub and Google OAuth for quick signup/signin
- **Passwordless Authentication**: Magic link authentication via email (production default)
- **Traditional**: Email/password authentication (local development default)
- **Environment-based**: Production uses passwordless by default, local development allows both methods
- **API Token Authentication**: For browser extensions and telegram bot integration

Authentication is handled with secure session management, CSRF protection, and Cloudflare Turnstile for bot protection.

For UI/UX, I'm going with a clean, simple, and intuitive design. The aesthetics should feel calm, cozy, and personal, with minimal interactions needed. The UI uses a black and white theme with modern flat design. While I don't have a specific inspiration yet, services like Pocket or Instapaper are somewhat similar, but I want something that feels more personal, and does not feel dead (Google products are a good example of dead feeling design).
Since the focus of the app is on text, I want black and white to be the main essence of the pages.
In the implementation I'm using HTML templates on the backend side and Tailwind CSS and HTMX on the front-end.

My current focus is on building a solid MVP, so consider the flexibility in your solutions. For example, I prefer a simpler design that allows me to change things faster in the future.

My questions will be mostly about technical aspects, UI/UX design, and ensuring the best user experience while maintaining a clean, aesthetic, and highly functional service.

Keep your answers concise and technical.

## Objectives of the application

The main idea of this web application is for you to no not lose what you've already found
Everything you've read, heard and watch should be indexed and searchable.
I got that idea first from the book "Moonwalking with Einstein" when I read about this guy who had a camera and was taking picture of all his daily experiences.
Later, I once found a fascinating article about mechanical watches with beautiful animations. I loved it so much and decided to read it later, but then when I searched for it, it was nowhere to be found and was buried under an all the irrelevant Google search results.
So I decided to make an app that helps to capture all the valuable information I find on the web.

## Design objectives
I wanted something that is not intrusive and embeds seamlessly into your workflow.
The app is quiet, and only interacts with you when you directly need it.
In the same time, it should help you to get a feel of achievement, for example by keeping track of the things you've read, or sending you weekly/monthly summary of your best reads.