# Design System for Pensieve

## Color Palette
- **Primary**: Blue-600 (#2563EB) - Buttons, links, icons
- **Primary Hover**: Blue-700 (#1D4ED8) - Button hover states
- **Primary Light**: Blue-100 (#DBEAFE) - Icon backgrounds
- **Text**: Gray-900 (#111827) - Headings and primary text
- **Text Muted**: Gray-600 (#4B5563) - Body text and descriptions
- **Background**: White (#FFFFFF) - Content areas
- **Background Subtle**: Gray-50 (#F9FAFB) - Page background
- **Border**: Gray-300 (#D1D5DB) - All borders and dividers
- **Interactive Hover**: Gray-300 (#D1D5DB) - Navigation hover states

## Status Colors

### Success (Emerald)
- **Primary**: Emerald-600 (#059669) - Success icons and main elements
- **Background**: Emerald-50 (#ECFDF5) - Success card backgrounds
- **Border**: Emerald-200 (#A7F3D0) - Success card borders
- **Text Dark**: Emerald-900 (#064E3B) - Success headings
- **Text Medium**: Emerald-800 (#065F46) - Success body text
- **Text Light**: Emerald-700 (#047857) - Success secondary text

### Error (Red)
- **Primary**: Red-600 (#DC2626) - Error icons and main elements
- **Background**: Red-50 (#FEF2F2) - Error card backgrounds
- **Border**: Red-200 (#FECACA) - Error card borders
- **Text Dark**: Red-900 (#7F1D1D) - Error headings
- **Text Medium**: Red-800 (#991B1B) - Error body text
- **Text Light**: Red-700 (#B91C1C) - Error secondary text

### Warning (Amber)
- **Primary**: Amber-600 (#D97706) - Warning icons and main elements
- **Background**: Amber-50 (#FFFBEB) - Warning card backgrounds
- **Border**: Amber-200 (#FDE68A) - Warning card borders
- **Text Dark**: Amber-900 (#78350F) - Warning headings
- **Text Medium**: Amber-800 (#92400E) - Warning body text
- **Text Light**: Amber-700 (#A16207) - Warning secondary text

## Layout & Spacing
- Max container width: `max-w-5xl mx-auto` (home), `max-w-7xl mx-auto` (header)
- Page padding: `px-6 py-8` (main content), `px-4 sm:px-6` (header)
- Section spacing: `mb-8` for major sections

## Component Styles

### Containers
- Hero/CTA sections: `bg-white rounded-lg border border-gray-300 p-8`
- Main content sections: `bg-white rounded-xl border border-gray-300 p-8`
- Alternative sections: `bg-gray-50 rounded-xl border border-gray-300 p-8`
- Header: `bg-gray-50 sticky top-0 z-50`
- Footer: `border-t border-gray-300 bg-gray-50`
- Dropdowns: `bg-white border border-gray-300 rounded-lg shadow-lg`
- Alerts: `bg-gray-50 border border-gray-300 rounded-lg`

### Status Cards
- Success cards: `bg-emerald-50 border border-emerald-200`
- Error cards: `bg-red-50 border border-red-200`
- Warning cards: `bg-amber-50 border border-amber-200`

### Typography
- Hero heading: `text-4xl font-bold tracking-tight text-gray-900`
- Main heading: `text-2xl font-bold tracking-tight text-gray-900`
- Section headings: `text-xl font-semibold text-gray-900`
- Subheadings: `text-lg font-medium text-gray-900`
- Body text: `text-xl text-gray-600` (hero), `text-gray-600` (regular)
- Navigation text: `text-sm font-medium`
- Helper text: `text-sm text-gray-600`
- Fine print: `text-xs font-medium text-gray-600`

### Buttons
- Primary CTA: `bg-blue-600 hover:bg-blue-700 text-white font-medium py-3 px-6 rounded-lg transition-colors`
- Secondary CTA: `bg-gray-900 hover:bg-gray-800 text-white font-medium py-3 px-6 rounded-lg transition-colors`
- Search button: `px-3 py-2 bg-gray-100 hover:bg-gray-300 border border-gray-300 rounded-lg`
- Navigation buttons: `px-3 py-2 text-gray-600 hover:text-gray-900 hover:bg-gray-50 rounded-lg transition-colors`
- Signup button: `px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors`
- Form buttons: `py-2 px-4 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700`
- Success buttons: `bg-emerald-600 hover:bg-emerald-700 text-white rounded-lg font-medium transition-colors`
- Destructive: `py-2 px-4 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700`

### Navigation
- Header height: `h-16`
- Logo: `text-xl font-semibold tracking-tight`
- Desktop nav: `hidden md:flex items-center gap-1`
- Mobile menu button: `md:hidden p-2`
- Account dropdown: `w-48` with `px-4 py-2` items

### Icons
- Logo icon: `h-5 w-5 text-blue-600`
- Feature icons: `h-6 w-6` in `h-12 w-12 rounded-md bg-blue-100 text-blue-600`
- Step numbers: `h-8 w-8 text-blue-600 font-bold` in `rounded-full bg-blue-100 p-3`
- Navigation icons: `w-4 h-4 text-gray-600`
- Mobile menu icon: `w-6 h-6`
- Social icons: `h-5 w-5`
- Profile placeholder: `h-32 w-32 rounded-full bg-blue-100 text-blue-600`
- Status icons: `w-5 h-5` (success: `text-emerald-600`, error: `text-red-600`, warning: `text-amber-600`)

### Interactive Elements
- Hover transitions: `transition-colors` or `transition-all`
- Links: `text-blue-600 hover:text-blue-700` with optional `hover:underline`
- Mobile menu toggle: Uses JavaScript to toggle `hidden` class

### Grid & Layout
- Feature grid: `grid md:grid-cols-2 gap-16`
- Steps grid: `grid md:grid-cols-3 gap-8`
- Personal story: `flex flex-col md:flex-row items-center gap-6`
- Photo placement: `md:w-1/4` and `md:w-3/4`

## Brand Elements
- App name: "Pensieve"
- Beta badge: `text-xs font-medium text-gray-600`
- Tagline emphasis on personal knowledge preservation
- Clean, minimal aesthetic with focus on readability
