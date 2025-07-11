# Project

## Define
Everything you read, hear, and watch should be indexed and searchable
Pensive is a web application helps you to seamlessly capture content without disrupting your workflow. To do so, it integrates with your browser and messaging apps.
It's kind of a bookmarking application, but it's also aware of the bookmark context.
The name is coming from Pensieve from the Harry Potter books. Just like in the book you could preseve your precious memories, in here you can have a collection of the knowledge you already gained.

## Prompt

In our conversations, consider the following context: I am developing a web service that helps users to index and preserve everything they read online. There are different way for users to specifiy a page as something they've read and want to find later. One example of it, is a browser extension: User can mark the pages they read to be indexed by clicking on the extension icon. Another way is to send the link to the Telegram bot of the application.
When we receive the link in our backend service, we will fetch the the full page content to a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they’ve saved.

When the knowledge is captured, user can search for content of what they have saved.

In the future, for premium users, I plan to support text extraction from YouTube videos for searchability. Also, we will use AI and LLMs to make sure we're extracting the best possible version of the website and provide better excerpt and summary to the user.

For UI/UX, I want a clean, simple, and intuitive design. The aesthetics should feel calm, cozy, and personal, with minimal interactions needed. The UI will use a mix of light and dark themes and may include small animations for a modern touch. While I don’t have a specific inspiration yet, services like Pocket or Instapaper are somewhat similar, but I want something that feels more personal, and does not feel dead (like Google products).
Since the focus of the app is on text, I want the black and white to be the main essence of the pages.
In the implementation I'm using HTML templates in the backend side and Tailwind CSS and HTMX on the front-end.

For future scalability, I plan to support public bookmarking and importing existing bookmarks, but my current focus is on building a solid MVP.

My questions will be mostly about technical aspects, UI/UX design, and ensuring the best user experience while maintaining a clean, aesthetic, and highly functional service.
