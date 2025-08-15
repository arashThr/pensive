# Linear-Inspired Design System for Pensive

## Typography
- **Font**: IBM Plex Sans (clean sans-serif for everything)
- **Headings**: Semibold (600) to Bold (700) weight
- **Body**: Regular (400) to Medium (500) weight
- **Links**: Medium weight with color change and underlines
- **Emphasis**: Medium/semibold weight with violet accent color

## Colors
- **Primary**: Violet (#8b5cf6)
- **Background**: White (#ffffff) and Light Gray (#f9fafb)
- **Text**: Gray-900 (#111827) for headings, Gray-600 (#4b5563) for body
- **Secondary**: Gray-200 (#e5e7eb) for borders
- **Success**: Green-600 (#059669) with Green-50 background (#ecfdf5)
- **Warning**: Amber-600 (#d97706) with Amber-50 background (#fef3c7)
- **Error**: Red-600 (#dc2626) with Red-50 background (#fef2f2)

## Layout
- **Rounded corners** - 8px (lg) for cards, 6px for buttons, 12px (xl) for major containers
- **Subtle shadows** - soft box shadows for cards and elevated elements
- **Thin borders** (1px) for most elements
- **Sophisticated contrast** - gray scale hierarchy with violet accents
- **Generous spacing** - balanced breathing room with modern proportions

## Components

### Buttons
- **Primary**: `bg-violet-600 text-white px-6 py-3 font-semibold rounded-lg hover:bg-violet-700 transition-colors`
- **Secondary**: `border border-gray-300 text-gray-700 px-6 py-3 font-semibold rounded-lg hover:bg-gray-50 transition-colors`
- **Danger**: `bg-red-600 text-white px-6 py-3 font-semibold rounded-lg hover:bg-red-700 transition-colors`
- **Sentence case** for button labels
- **Smooth transitions** for hover states

### Containers
- **Cards**: `bg-white border border-gray-200 rounded-xl shadow-sm p-6`
- **Info boxes**: `bg-gray-50 border border-gray-200 rounded-lg p-6`
- **Success**: `bg-green-50 border border-green-200 rounded-lg p-6 text-green-800`
- **Warning**: `bg-amber-50 border border-amber-200 rounded-lg p-6 text-amber-800`
- **Error**: `bg-red-50 border border-red-200 rounded-lg p-6 text-red-800`

### Navigation
- **Header**: `bg-white/80 backdrop-blur-sm border-b border-gray-200`
- **Footer**: `border-t border-gray-200`
- **Sentence case** for navigation items
- **Color and background changes** on hover with smooth transitions

### Content Hierarchy
- **H1**: `text-4xl font-bold text-gray-900 mb-4`
- **H2**: `text-3xl font-bold text-gray-900 mb-6`
- **H3**: `text-xl font-semibold text-gray-900 mb-4`
- **Body**: `text-gray-600 leading-relaxed`
- **Links**: `text-violet-600 hover:text-violet-700 font-medium transition-colors`

### Interactive Elements
- **Smooth transitions** - `transition-colors` and `transition-all` for polished feel
- **Color changes** for links with optional underlines
- **Violet accents** for interactive elements
- **Hover states**: Background, border, and color changes with smooth transitions

### Loading States & Animations
- **Spinning indicators**: `animate-spin` with violet border-top for modern spinners
- **Smooth transitions**: 200ms ease for most hover states
- **Gradient backgrounds**: Subtle gradients for hero sections and special areas

## Content Principles
- **Direct language** - no marketing fluff
- **Technical honesty** - explain exactly what it does
- **Personal voice** - "I built this because..."
- **Real examples** - actual search queries, real numbers
- **No corporate speak** - avoid "seamlessly," "powerful," etc.

## Visual Hierarchy
1. **Typography weights** and **colors** create hierarchy
2. **Spacing and borders** indicate importance and grouping
3. **Violet accents** draw attention to key interactive elements
4. **Sentence case** for most UI elements
5. **Gray scale** for content hierarchy (900 for headings, 600 for body)

## Design Principles (Things to Embrace)
- ✅ Rounded corners for modern feel
- ✅ Subtle shadows for depth
- ✅ Careful use of gradients
- ✅ Violet accent color for interactivity
- ✅ Smooth transitions for polish
- ✅ SVG icons when they add clarity
- ✅ Sophisticated hover effects
- ✅ Direct, honest language (avoid corporate speak)

## Grid & Spacing
- **Responsive max-width**: `max-w-5xl mx-auto` for main content, `max-w-6xl` for library
- **Consistent padding**: `px-6` to `px-8` horizontally, `py-8` to `py-16` vertically
- **Vertical rhythm**: `mb-6` to `mb-16` for sections based on importance
- **Modern layouts**: Card grids, flexbox, and responsive design patterns
- **Generous whitespace**: Balanced spacing that feels modern and breathable