# AI prompts

## Project

In our conversations, consider the following context: I am developing a web service that helps users gather and search through all their bookmarks in one place. Users add pages by bookmarking the page in their browser. This action is captured via our browser extension, which sends the full page content to a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they’ve saved.

Initially, bookmarks will support user-defined tags that are assigned in the browser when saving the bookmark, with future plans for more organization features. Users will be able to edit tags and add notes to saved bookmarks later. In the future, I also plan to support text extraction from YouTube videos for searchability.

The service will include API integrations via API keys, such as a Telegram bot, which will not only add bookmarks but also support search and retrieval. API tokens will be user-specific.

For UI/UX, I want a clean, simple, and intuitive design that requires no tutorials. The aesthetics should feel calm, cozy, and personal, with minimal interactions needed. The UI will use a mix of light and dark themes and may include small animations for a modern touch. While I don’t have a specific inspiration yet, services like Pocket or Instapaper are somewhat similar, but I want something that feels more personal.

For future scalability, I plan to support public bookmarking and importing existing bookmarks, but my current focus is on building a solid MVP.

My questions will be mostly about technical aspects, UI/UX design, and ensuring the best user experience while maintaining a clean, aesthetic, and highly functional service.

## Design

AI prompt to generate the design:

```
I’m building a Go web application with HTML templates and Tailwind CSS, aiming for a **Minimal Monochrome** design—clean, text-focused, professional, and approachable. I’ll provide the raw HTML for a page, and I need you to style it to match this design:

- **Palette**: Black (#000) for text, white (#fff) background, subtle grays (#333, #999) for secondary elements. Use blue (#3b82f6, Tailwind’s `blue-600`) as the only accent color for CTAs (buttons, links), with `hover:bg-blue-500` or `hover:text-blue-500` for interactivity.
- **Typography**: Use `font-sans` with the Inter font (via Google Fonts: `<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600&display=swap" rel="stylesheet">`). Headers are `font-semibold`, text is `text-sm` or `text-base`, no underlines on links—just color shifts.
- **Layout**: Centered content with `max-w-md` or `max-w-xl` for forms/content blocks, padding like `p-6` or `px-6 py-8`, and `flex justify-center` for alignment. Minimal spacing (`py-3`, `py-4`) for breathing room without clutter.
- **Forms**: Inputs have `border border-gray-300`, `rounded`, `px-3 py-2`, `text-black`, `placeholder-gray-500`, and `focus:border-blue-500 outline-none transition-all`. Labels are `block text-sm text-gray-700 mb-1` above inputs. Buttons are `w-full py-3 bg-blue-600 hover:bg-blue-500 text-white rounded text-base font-semibold`.
- **Header/Footer**: Assume they’re provided via `{{template "header" .}}` and `{{template "footer" .}}`, styled as white with `border-gray-200` borders, black text, and blue CTAs (as in previous chats).
- **Principles**: Keep it uncluttered, text-centric, and professional. Avoid shadows, gradients, or extra colors unless justified. Prep for subtle animations with `transition-all` on interactive elements.

Here’s my raw HTML:
[Insert your raw HTML here]

Please update it with Tailwind classes to match this design. Keep the structure intact, adjust only styling, and ensure it works with Go templating (e.g., `{{csrfField}}`). If anything’s unclear, ask me specific questions.
```
