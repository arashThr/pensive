# AI prompts

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
