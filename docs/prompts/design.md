# Brutalist Design System for Pensive

## Typography
- **Font**: IBM Plex Mono (monospace for everything)
- **Headings**: Bold, no fancy sizing - just font-bold
- **Body**: Regular weight
- **Links**: Underlined, no color change
- **Emphasis**: Bold text, not color

## Colors
- **Primary**: Black (#000000)
- **Background**: White (#FFFFFF) 
- **Secondary Background**: Gray-100 (#F3F4F6)
- **Accent**: None - use bold text instead
- **Borders**: Black, 2px thick for important elements, 1px for minor
- **Warning**: Yellow-100 background with black border
- **Error**: Red-100 background with red-600 border

## Layout
- **No rounded corners** - everything is square
- **No shadows** - flat design only
- **Thick borders** (2px) for major sections
- **Sharp contrast** - black on white, no grays for text
- **Wide spacing** - plenty of breathing room

## Components

### Buttons
- **Primary**: `bg-black text-white px-6 py-3 font-bold hover:bg-gray-800`
- **Secondary**: `border-2 border-black text-black px-4 py-2 font-bold hover:bg-gray-100`
- **All caps text** for button labels
- **No rounded corners** - sharp rectangular buttons only

### Containers
- **Main sections**: `border-2 border-black p-6`
- **Emphasis boxes**: `border-l-4 border-black pl-6`
- **Warnings**: `border-2 border-black p-6 bg-yellow-100`
- **No rounded corners anywhere**

### Navigation
- **Header**: `border-b-2 border-black`
- **Footer**: `border-t-2 border-black`
- **All caps** for navigation items
- **Underline on hover** instead of color changes

### Content Hierarchy
- **H1**: `text-4xl font-bold` (no color, just size)
- **H2**: `text-2xl font-bold border-b-2 border-black pb-2`
- **H3**: `font-bold text-lg`
- **Lists**: Simple bullets (•) or numbers, no fancy styling

### Interactive Elements
- **No smooth transitions** - instant state changes
- **Underlines** for links, not color changes
- **Bold text** for emphasis instead of colors
- **Hover states**: Simple background color changes

### Loading States & Animations
- **Spinning wheel**: Use `animate-spin` for loading indicators

## Content Principles
- **Direct language** - no marketing fluff
- **Technical honesty** - explain exactly what it does
- **Personal voice** - "I built this because..."
- **Real examples** - actual search queries, real numbers
- **No corporate speak** - avoid "seamlessly," "powerful," etc.

## Visual Hierarchy
1. **Bold typography** creates hierarchy, not colors
2. **Border thickness** indicates importance
3. **Spacing** separates sections
4. **ALL CAPS** for UI elements (buttons, nav)
5. **Regular case** for content

## Anti-Patterns (Things to Avoid)
- ❌ Rounded corners
- ❌ Drop shadows
- ❌ Gradients
- ❌ Multiple colors for decoration
- ❌ Smooth animations
- ❌ Icon fonts or SVG icons
- ❌ Fancy hover effects
- ❌ Corporate marketing language

## Grid & Spacing
- **Fixed max-width**: `max-w-4xl mx-auto`
- **Consistent padding**: `px-6` or `px-8`
- **Vertical rhythm**: `mb-8` or `mb-16` for sections
- **Simple grid**: Only basic 2-column layouts when needed