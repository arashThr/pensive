# UI Design Principles

This document is the source of truth for building and updating pages. It reflects the current minimalist black/white/gray direction. Always read this before touching templates.

---

## Color System

All colors are defined as CSS variables in `tailwind/style.css` and exposed as utility classes. **Never reach for raw Tailwind color names** (`text-white`, `bg-gray-900`, `text-slate-400`, `text-emerald-300`, etc.) — use only the theme utilities below.

| Variable | Value | Utility classes |
|---|---|---|
| `--color-bg` | `#000000` | `bg-main` |
| `--color-bg-secondary` | `#181818` | `bg-secondary` |
| `--color-border` | `#222222` | `border-main` |
| `--color-text` | `#ffffff` | `text-main` |
| `--color-text-secondary` | `#b3b3b3` | `text-secondary` |
| `--color-accent` | `#ffffff` | `accent` (color only) |

Hover variants are also utility classes: `hover:bg-main`, `hover:bg-secondary`, `hover:text-main`, `hover:text-secondary`.

---

## Surface Hierarchy

Use two background levels to create depth without extra colors:

- **Page background**: `bg-main` (`#000`) — the canvas
- **Raised surface**: `bg-secondary` (`#181818`) — cards, feature boxes, callout sections, integration cards
- **Recessed surface**: `bg-main` inside a `bg-secondary` section — creates a "cut in" feel (e.g. pricing premium card)

Rule: sections that need to stand out from the page use `bg-secondary`. Sections that need to recede (or are inside a raised block) use `bg-main`.

---

## Borders

Always `border border-main`. No opacity variants like `border-white/10`. `<hr>` dividers also use `border-main`.

---

## Typography

- Font: IBM Plex Sans (loaded globally)
- Hero heading: `text-4xl sm:text-5xl font-bold tracking-tight text-main`
- Section heading: `text-xl font-semibold text-main`
- Card heading: `text-base font-semibold text-main`
- Body/description: `text-sm text-secondary`
- Muted label (e.g. pricing tier): `text-sm text-secondary uppercase tracking-wider`

---

## Buttons

Two types only:

### Outlined (primary action)
```html
<a href="..." class="inline-flex items-center justify-center rounded-lg border border-main bg-main px-5 py-3 font-semibold text-main hover:bg-secondary transition-colors">
  Label
</a>
```
Use for: primary CTA, "Get started", "Sign up", "Try it free", header "Sign up", header "Account".

### Ghost (secondary action)
```html
<a href="..." class="font-medium text-secondary hover:text-main transition-colors">
  Label →
</a>
```
Use for: "Sign in" alongside a primary button, "Learn more" links.

**Rules:**
- A button starting on `bg-main` must hover to `bg-secondary`, and vice versa. Never `hover:bg-main` on a `bg-main` element (no visual change).
- The header nav must look the same whether the user is logged in or out. Both states expose exactly one outlined button (logged-out: "Sign up"; logged-in: "Account") plus plain text links.
- No colored buttons. No `bg-violet-*`, `bg-blue-*`, etc.

---

## Header Nav

**Logged out:**
```
[Logo]          [Sign in (ghost)]  [Sign up (outlined)]
```

**Logged in:**
```
[Logo]    [Home]  [Extensions]  [Account (outlined + dropdown)]
```

The outlined button is always the rightmost item and is the primary action for that auth state.

---

## Cards & Sections

Standard card:
```html
<div class="rounded-xl border border-main bg-secondary p-6">
  ...
</div>
```

Sections with visual blocks (podcast feature, CTA, pricing Free tier):
- Use `bg-secondary` to lift them off the page background.

Sections that are logically "inside" a raised block (pricing Premium card):
- Use `bg-main` so they read as distinct from their container.

---

## Links in Prose

Links inside paragraphs (e.g. "Why I built this"):
```html
<a class="text-main underline hover:no-underline transition-all" href="...">text</a>
```
Never use colored variants (`text-violet-200`, etc.) for inline links.

---

## List Items / Feature Lists

Use em-dash (`—`) for bullet-style lists inside cards. No SVG checkmarks for simple lists.
```html
<ul class="mt-4 space-y-1 text-sm text-secondary">
  <li>— Item one</li>
  <li>— Item two</li>
</ul>
```

---

## Alerts

Alerts are not color-coded. Both error and info use the same neutral surface:
```html
<div class="bg-secondary border border-main text-main p-4 rounded-lg ...">
```

---

## What to Avoid

- Raw Tailwind color names: `text-white`, `text-slate-*`, `bg-gray-*`, `text-emerald-*`, `text-violet-*`, `border-white/*`, etc.
- Inline `style="..."` for colors — only the `background-color` body override on `<body>` is acceptable.
- `bg-white/5`, `border-white/10` opacity variants.
- SVG icon badges inside bullet lists — use plain text dashes.
- `shadow-lg` on buttons — no shadows.
- Per-page background blobs or gradients.
- Buttons with no visible hover state (`hover:bg-main` on a `bg-main` button).
