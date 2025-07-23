# Project

## Define
Build your own searchable memory of the web
Everything you read, hear, and watch should be indexed and searchable
Pensive is a web application helps you to seamlessly capture content without disrupting your workflow. To do so, it integrates with your browser and messaging apps.
It's kind of a bookmarking application, but it's also aware of the bookmark context.
The name is coming from Pensieve from the Harry Potter books. Just like in the book you could preseve your precious memories, in here you can have a collection of the knowledge you already gained.

## Prompt

In our conversations, consider the following context: I am developing a web service that helps users to index and preserve everything they read online. There are different ways for users to specify a page as something they've read and want to find later. One example, and the main way, is a browser extension: the User can mark the pages they read to be indexed by clicking on the extension icon. Another way is to send the link to the Telegram bot of the application.
When we receive the link in our backend service, we will fetch the full page content to a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they’ve saved.
In the case of the extension, we send the whole HTML and metadata of the page to avoid scraping issues with sites like
Reddit and Stack Overflow.

When the knowledge is captured, user can search for content of what they have saved.
We also provide premium feature which is mostly using LLMs for getting better content: We use Google Gemini Flash lite
to create markdown version of the page and generate summary and tags.

Premium features will be extended in the future, including the option to support text extraction from YouTube videos for searchability. Also, I want to add the option to send weekly/monthly summary of the highlights of your readings.

For UI/UX, I'm going to go with a clean, simple, and intuitive design. The aesthetics should feel calm, cozy, and personal, with minimal interactions needed. The UI will use a mix of light and dark themes. We keep things flat and simple with a modern look. While I don’t have a specific inspiration yet, services like Pocket or Instapaper are somewhat similar, but I want something that feels more personal, and does not feel dead (Google products are a good example of dead feeling design).
Since the focus of the app is on text, I want the black and white to be the main essence of the pages.
In the implementation I'm using HTML templates in the backend side and Tailwind CSS and HTMX on the front-end.

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