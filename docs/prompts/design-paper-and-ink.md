## Pensive Design System - Paper & Ink

Context: You are helping redesign pages for Pensive, a personal knowledge indexing web application. The user has established a design system (below) that should be applied consistently to all pages and modifications. Use this system as the foundation for any design work, adapting it appropriately to the specific page requirements while maintaining consistency.

**Color Palette:**
- Primary: `#2d2d2d` (charcoal) - main text
- Secondary: `#fefefe` (paper white) - background
- Accent: `#8b7355` (warm brown) - highlights, links, accents
- Subtle: `#f5f5f5` (off-white) - card backgrounds, subtle sections
- Supporting: `#gray-600` for secondary text, `#gray-200` for borders

**Typography:**
- Font: Inter (400, 500, 600, 700 weights)
- Hierarchy: Large headlines (text-5xl), section headers (text-2xl), body (text-xl/base)
- Antialiased rendering

**Design Principles:**
- **Minimal & Clean**: Strip away Bootstrap-style cards, excessive shadows, heavy borders
- **Personal & Warm**: Copy should feel like one person built this, not corporate
- **Easy to Iterate**: Simple class structure, avoid complex component nesting
- **Text-Focused**: Black/white essence, content hierarchy over decoration
- **Non-Intrusive**: Messaging should feel helpful, not invasive or scary

**Layout Standards:**
- Max-width: `max-w-4xl` for main content
- Padding: `px-6` horizontal, `py-16` for main sections
- Spacing: `mb-20` between major sections, `mb-12` for subsections
- Grid: Simple 2-3 column layouts, generous gaps (`gap-12-16`)

**Component Style:**
- Buttons: `px-8 py-4`, `rounded-lg`, charcoal bg with paper text
- Cards: Minimal - `bg-subtle`, `rounded-lg`, `p-8`, no shadows
- Links: `text-warm-brown`, `hover:underline` for inline links
- Icons: Simple geometric shapes or numbered circles, warm-brown accents

**Tone & Copy Guidelines:**
- Personal story-driven (mechanical watch article example)
- Focus on "never lose what you've found" not technical features
- Avoid scary language about "extraction" or "indexing"
- Use "preserve," "capture," "remember" instead
- Emphasize workflow integration, not disruption

**Technical Constraints:**
- Use Tailwind CSS utility classes
- Compatible with Go templates
- Responsive-first approach
- Performance-conscious (minimal DOM complexity)

**Open Decisions:**
- Animation preferences (subtle vs noticeable)
- Specific page layouts beyond homepage
- Navigation structure details
- Build process optimization approach
