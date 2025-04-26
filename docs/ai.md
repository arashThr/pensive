# AI prompts

## Project

In our conversations, consider the following context: I am developing a web service that helps users gather and search through all their bookmarks in one place. Users add pages by bookmarking the page in their browser. This action is captured via our browser extension, which sends the full page content to a Go-based backend. We extract and store the content in PostgreSQL with full-text search capabilities. Users can later search through everything they’ve saved.

Initially, bookmarks will support user-defined tags that are assigned in the browser when saving the bookmark, with future plans for more organization features. Users will be able to edit tags and add notes to saved bookmarks later. In the future, I also plan to support text extraction from YouTube videos for searchability.

The service will include API integrations via API keys, such as a Telegram bot, which will not only add bookmarks but also support search and retrieval. API tokens will be user-specific.

For UI/UX, I want a clean, simple, and intuitive design. The aesthetics should feel calm, cozy, and personal, with minimal interactions needed. The UI will use a mix of light and dark themes and may include small animations for a modern touch. While I don’t have a specific inspiration yet, services like Pocket or Instapaper are somewhat similar, but I want something that feels more personal.
Since the focus of the app is on text, I want the black and white to be the main essence of the pages.
In the implementation I'm using HTML templates in the backend side and Tailwind CSS and HTMX on the front-end.

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

### New design

To: Future Grok (March 14, 2025 onwards)
Subject: Replicating Pensieve Bookmarking App Page Design
Hey future me! I’m helping a user build a web service called "Pensieve" that lets users save and search bookmarks via a browser extension, with a Go backend, PostgreSQL storage, and full-text search. We’re designing the front-end pages, and here’s exactly how I’ve been modifying their HTML templates to make them production-ready, functional, and aligned with their vision. Follow this to the letter for any new pages they provide—no deviations unless they explicitly ask!
Context
App Purpose: Users bookmark pages via a browser extension; full page content is sent to a Go backend, stored in PostgreSQL, and searchable later. Tags, notes, and future features like YouTube text extraction are planned. MVP focus is on core bookmarking and search.

Tech Stack: Go backend with HTML templates, Tailwind CSS for styling, HTMX for dynamic interactions (used minimally so far—add only if requested). Pages are served as full HTML, not SPA.

Aesthetic Goals: Crisp text, slick look, clutter-free but not empty/dead, cozy and personal vibe. Text is the essence (black-and-white focus), with minimal interactions and subtle animations for modernity.

UI/UX: Clean, simple, intuitive. Light theme (with dark theme potential later). Small animations like hovers and focus states. Inspired by Pocket/Instapaper but more personal.

Design Decisions
Color Palette (Tailwind Defaults):
gray-100 (#f3f4f6): Body background—soft, calm base.

gray-900 (#111827): Primary text—crisp, dark, readable.

red-600 (#dc2626): Primary accent—buttons, hovers, focus states (warm and bold).

gray-500 (#6b7280): Secondary text—labels, placeholders, links (subtle).

amber-700 (#b45309): Minor accent—alerts (cozy warmth, used sparingly).

No custom hex colors—stick to Tailwind naming (e.g., text-gray-900, not text-[#111827]).

Typography:
Font: Inter (loaded via <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">).

Classes: font-sans antialiased on <body> for smooth rendering.

Weights: font-semibold for headings/buttons, font-medium for nav links, regular for body text.

Sizes: text-4xl for home page heading, text-2xl for form headings, text-base for buttons, text-sm for labels/links, text-xs for footer-like small text.

Tracking: tracking-tight on headings for a modern, compact feel.

Layout Rules:
Body: min-h-screen bg-gray-100 text-gray-900 flex flex-col (content grows between header/footer).

Header: White background, border-b border-gray-200 shadow-sm, max-w-7xl mx-auto for content width, px-6 py-4 padding.

Footer: White background, border-t border-gray-200, max-w-7xl mx-auto, py-6 padding.

Main Content: Centered with flex justify-center, py-16 outer padding, inner container at max-w-md w-full (forms) or max-w-2xl (home page).

Forms: White card with bg-white rounded-lg shadow-sm, p-8 padding, py-4 between fields, py-6 before buttons.

Spacing: Generous but balanced—use py-12/py-16 for outer sections, py-4/py-6 inside forms, mb-2 for label-input gaps.

Components:
Inputs: 
w-full px-4 py-2 border border-gray-300 focus:border-red-600 focus:ring-1 focus:ring-red-600/20 outline-none bg-white text-gray-900 rounded-md shadow-sm transition-all duration-200 placeholder:text-gray-500.

Full width, subtle border, red focus with faint ring, smooth transitions.

Buttons: 
w-full py-3 bg-red-700 hover:bg-red-600 text-white rounded-md text-base font-semibold shadow-sm transition-all duration-200.

Full-width, red accent, hover shift, subtle shadow.

Links: 
text-gray-500 hover:text-red-600 transition-colors.

Subtle base color, red on hover, smooth transition.

Nav Links: 
Desktop: text-sm font-medium hover:text-red-600 transition-colors, space-x-6.

Mobile: block px-6 py-3 hover:bg-gray-100 transition-colors.

Alerts: 
flex px-4 py-3 mb-2 bg-white border border-red-500 text-red-600 rounded-md shadow-sm (error) or border-amber-700 text-amber-700 (success).

Close icon with hover:opacity-75 transition-opacity.

Animations:
Subtle and modern: transition-all duration-200 on inputs/buttons for focus/hover, transition-colors on links for color shifts, transition-opacity on alert close.

No over-the-top effects—keep it minimal and slick.

Header/Footer Specifics:
Header: 
Logo: text-2xl font-semibold tracking-tight, with emoji () and "Pensieve" in a flex row.

Nav: Conditional—logged in: "Bookmarks", "Account", "Integrations", "Sign Out"; logged out: "Sign In", "Sign Up" button.

Mobile: Hamburger toggles menu with hidden md:hidden and JS toggle.

Footer: 
text-gray-500, centered, "© 2025 Pensieve" + "Privacy" | "Terms" links.

Modification Instructions
When they give you an HTML template:
Structure Check:
Ensure it starts with {{template "header" .}} and ends with {{template "footer" .}}.

Wrap main content in <div class="py-16 flex justify-center"> with an inner <div class="p-8 max-w-md w-full bg-white rounded-lg shadow-sm"> for forms, or max-w-2xl for wider content (e.g., home page).

Apply Colors:
Replace all text colors with gray-900 (primary) or gray-500 (secondary).

Buttons: bg-red-700 hover:bg-red-600 text-white.

Links: text-gray-500 hover:text-red-600.

Inputs: border-gray-300 focus:border-red-600 focus:ring-1 focus:ring-red-600/20 placeholder:text-gray-500.

Update Typography:
Headings: text-2xl font-semibold text-gray-900 tracking-tight (forms) or text-4xl (home).

Labels: text-sm text-gray-500 mb-2.

Buttons: text-base font-semibold.

Links: text-sm text-gray-500.

Adjust Layout:
Add py-16 to outer wrapper, p-8 to inner card.

Form fields: py-4 between, py-6 before button.

Flex layouts (e.g., link rows): flex justify-between.

Enhance Components:
Inputs: Add full styling as above, ensure rounded-md shadow-sm.

Buttons: Full-width, styled as above, centered in py-6.

Links: Apply hover and transition classes.

Keep HTMX Minimal:
Only add HTMX if they ask (e.g., hx-post for form submissions with inline feedback). Current pages are static POSTs.

Example Transformations
Input:
Old: <input class="w-full px-3 py-2 border border-gray-300 rounded text-black placeholder-gray-500 focus:border-blue-500 outline-none">
New: <input class="w-full px-4 py-2 border border-gray-300 focus:border-red-600 focus:ring-1 focus:ring-red-600/20 outline-none bg-white text-gray-900 rounded-md shadow-sm transition-all duration-200 placeholder:text-gray-500">

Button:
Old: <button class="w-full py-3 bg-blue-600 hover:bg-blue-500 text-white rounded text-base font-semibold">
New: <button class="w-full py-3 bg-red-700 hover:bg-red-600 text-white rounded-md text-base font-semibold shadow-sm transition-all duration-200">

Final Notes
Stick to this exact palette and styling unless they request changes (e.g., “use blue instead”).

If they provide a page without header/footer, assume it’s a full page and add them.

Double-check spacing—it’s key to the cozy, clutter-free vibe.

If unsure, ask them: “Want any tweaks to colors, spacing, or HTMX?” before proceeding.

Now, take their next HTML, apply these rules, and keep Pensieve looking slick and consistent! They’ll love it.

