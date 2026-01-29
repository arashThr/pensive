# Design System (Current) — Clean Dark + Token-Friendly

This document captures the *current* UI decisions we’ve converged on while simplifying the templates. It is the source of truth for building new pages and for updating older pages that were written with the previous “Kandinsky circles / lots of gradients / heavy blur” mindset.

The goals are:

- Keep the dark, modern feel.
- Reduce visual noise and custom one-off styling.
- Prefer a small, consistent palette and default utility patterns.
- Make it easy to swap in tokens later (colors/spacing/typography).
- Preserve functionality (menus, alerts, slider) while keeping markup lean.

## Where This Lives In Code

- Global layout / header / footer / background: `web/templates/tailwind.gohtml`
- Landing page content + slider: `web/templates/home.gohtml`
- Static assets (icons, etc): `web/assets/**` (ex: `/assets/icons/telegram.svg`)

### Tailwind Mode

- Production uses `/assets/style.css`.
- Non-production uses Tailwind CDN with minimal config (font family).

## Visual Direction

### Overall Look

- Dark base, “quiet” surfaces.
- One primary accent (violet) + neutral slate.
- Background decoration exists but is minimal and non-dominant.

### Background Decoration

We intentionally keep background blobs subtle and few:

- Put them in the base template (not per-page).
- No per-page custom gradients.
- Use simple Tailwind utilities instead of inline `style="..."`.

Current pattern (conceptual):

- Fixed background layer
- 2–3 blurred blobs
- Low opacity (`/5`–`/10`)

## Typography

- Font: IBM Plex Sans (already loaded globally).
- Headings: `font-semibold` / `font-bold`.
- Body: `text-slate-300` for readable muted text.
- Avoid multi-color gradient text; use a single accent span when needed.

Recommended scale:

- Page hero: `text-4xl sm:text-5xl font-bold tracking-tight`
- Section titles: `text-xl font-semibold`
- Card titles: `text-base font-semibold` or `text-lg font-semibold`
- Body: `text-sm`–`text-base`

## Color System

### Neutrals (Primary)

- Background: `from-slate-950 via-slate-900 to-slate-950`
- Text: `text-slate-100` / `text-white`
- Muted text: `text-slate-300`, `text-slate-400`
- Borders: `border-white/10`
- Surfaces: `bg-white/5`

### Accent (One Primary)

- Accent text: `text-violet-200`
- Accent button: `bg-violet-600 hover:bg-violet-500`
- Accent surface: `bg-violet-500/10` with `border-violet-500/30` (use sparingly)

### Alerts (Dark-Friendly)

- Error: `bg-red-500/10 border border-red-500/30 text-red-200`
- Info/success-ish: `bg-sky-500/10 border border-sky-500/30 text-sky-200`

## Spacing & Layout Rhythm

We intentionally increased spacing so the first look doesn’t feel cramped.

### Page Container

Use this wrapper on content pages:

- `max-w-6xl mx-auto`
- `px-4 sm:px-6 lg:px-8`
- `py-16 sm:py-20`

### Section Spacing

Standard:

- `mt-16 sm:mt-20`

Avoid stacking many small sections at `mt-8` unless the page is very short.

### Grids & Gaps

- Hero: `gap-12 lg:gap-14`
- Standard grid: `gap-6`
- Prefer fewer, larger blocks over many tiny boxes.

## Reusable Building Blocks

### Surface Card

Base surface:

- `rounded-xl border border-white/10 bg-white/5`
- `p-6 sm:p-8`

Use this for: integrations, feature cards, pricing cards, callouts.

### Primary Button

- `rounded-lg bg-violet-600 px-5 py-3 font-semibold text-white hover:bg-violet-500 transition-colors`

### Secondary Button

- `rounded-lg border border-white/10 bg-white/5 px-5 py-3 font-semibold text-white hover:bg-white/10 transition-colors`

### Inline Links

- `text-violet-200 hover:text-white transition-colors`

## Recommended Home Page Narrative Order

For landing pages, the current “good flow” is:

1. Hero (headline + short description + CTAs)
2. Integrations (“Save from anywhere”) as a prominent single block
3. Screenshots/slider
4. “Everything you need…” feature grid
5. “Why I built this” story
6. Pricing (Self-host + Hosted Free + Hosted Premium)
7. Final CTA

This makes integrations feel important *without* interrupting the story.

## Integrations Pattern

We combine extensions + Telegram into one cohesive integrations card:

- Left side: Chrome + Firefox store badges.
- Right side: Telegram using `/assets/icons/telegram.svg`.
- Include one clear CTA (“Open bot”) and a secondary CTA (“See integrations”).
- Avoid random unrelated inline SVG icons here.

## Feature Grid Pattern (“Everything you need…”)

4 cards is the sweet spot:

- Search
- Save
- AI
- Open source

Rules:

- Each card can have a small icon (simple inline SVG) + title.
- Keep lists short (3–5 bullets).
- Avoid heavy shadows and `backdrop-blur` unless there’s a strong reason.

## Pricing Pattern

We show 3 options:

- Self-host ($0): “unlimited usage” framing + GitHub link.
- Hosted Free ($0): daily limits.
- Hosted Premium ($5): more headroom.

Rules:

- Use a real bullet list (not dense paragraphs).
- Keep “Premium” slightly highlighted using violet border/surface.
- Don’t use “POPULAR” badges or gradients unless there’s clear product intent.

## Slider / Interactive UI Guidelines

- Keep JS minimal and scoped to the feature.
- Accessibility:
  - Buttons must have `aria-label`.
  - Keyboard support for arrows (left/right).
  - Touch support for swipe.
- Avoid complex DOM structures: prefer a small set of IDs/classes.

## Do / Don’t

### Do

- Use `border-white/10` and `bg-white/5` for most surfaces.
- Keep accents mostly in CTAs and links.
- Use consistent spacing (`mt-16 sm:mt-20`, `p-6 sm:p-8`).
- Prefer global decoration in `tailwind.gohtml`.
- Keep markup easy to scan and extend.

### Don’t

- Don’t add many per-page blobs/circles.
- Don’t reintroduce rainbow gradients or multi-accent palettes.
- Don’t use heavy blur/shadow layers everywhere.
- Don’t hardcode inline styles for gradients.

## Notes for Future Tokenization

When we introduce design tokens, the following are the “first targets” to replace with tokens:

- Neutral surfaces: `bg-white/5`, `border-white/10`
- Text: `text-slate-300`, `text-slate-400`
- Accent: `violet-600` / `violet-500` / `violet-200`
- Spacing: `mt-16 sm:mt-20`, `p-6 sm:p-8`, `py-16 sm:py-20`

If a new page can’t be built using the primitives above, treat that as a signal to *extend the system*, not to add page-specific styling.