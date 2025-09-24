# Design System - Kandinsky "Several Circles" Theme

This document outlines the complete design system inspired by Wassily Kandinsky's "Several Circles" painting, implemented across the Go web application. This guide provides all necessary information for any developer or AI agent to recreate and extend this design consistently.

## Design Philosophy

The theme captures Kandinsky's sophisticated dark aesthetic with floating geometric elements, creating a modern, playful yet professional interface. Key principles:

- **Dark sophistication**: Deep backgrounds with subtle gradients
- **Floating elements**: Circular shapes with radial gradients that animate and layer
- **High contrast**: Ensuring excellent readability with proper text-background contrast
- **Mobile-first**: All designs prioritize small screen experiences
- **Technical integrity**: Maintain original technical messaging while enhancing visual appeal

## Typography
- **Font**: IBM Plex Sans (clean sans-serif for everything)
- **Headings**: Semibold (600) to Bold (700) weight
- **Body**: Regular (400) to Medium (500) weight
- **Links**: Medium weight with color change and underlines
- **Emphasis**: Medium/semibold weight with violet accent color

## Color Palette

### Primary Background Colors
```css
/* Main body gradient */
bg-gradient-to-br from-gray-900 via-slate-800 to-gray-900

/* Section backgrounds with transparency */
bg-white/10 backdrop-blur-sm
bg-gray-800/50 backdrop-blur-sm
bg-gray-900/80 backdrop-blur-sm
```

### Text Colors
```css
/* Primary text on dark backgrounds */
text-white

/* High contrast for readability */
text-gray-100

/* Muted text */
text-gray-300

/* Interactive elements */
text-indigo-300
text-orange-400
```

### Accent Colors for Circles
```css
/* Primary circles - large floating elements */
from-blue-500/30 to-blue-700/20
from-red-500/40 to-orange-600/30
from-yellow-400/35 to-amber-500/25
from-green-500/45 to-emerald-600/35
from-purple-500/30 to-violet-600/20

/* Secondary circles - smaller accents */
from-pink-500/50 to-rose-600/40
from-indigo-500/60 to-blue-600/50
from-teal-500/55 to-cyan-600/45
```

### Interactive Element Colors
```css
/* Primary buttons */
bg-gradient-to-br from-red-400 to-orange-500
hover:from-red-500 hover:to-orange-600

/* Secondary buttons */
bg-gradient-to-r from-orange-500 to-red-500
hover:from-orange-600 hover:to-red-600

/* Input focus states */
focus:border-orange-400 focus:ring-orange-200/50
```

## Layout Patterns

### Container Structure
```html
<!-- Page wrapper with overflow control -->
<div class="relative min-h-screen overflow-hidden">

  <!-- Floating circles background layer -->
  <div class="absolute inset-0 pointer-events-none">
    <!-- Circle elements here -->
  </div>

  <!-- Content overlay with proper z-index -->
  <div class="relative z-10 px-6 py-12 max-w-5xl mx-auto">
    <!-- Page content -->
  </div>
</div>
```

### Responsive Spacing
```css
/* Mobile-first padding */
px-4 py-8        /* Small screens */
md:px-6 md:py-12 /* Medium screens and up */
lg:px-8 lg:py-16 /* Large screens */

/* Section spacing */
mb-8 md:mb-12    /* Section bottom margins */
gap-6 md:gap-8   /* Grid/flex gaps */
```

## Floating Circles System

### Large Primary Circles (Background Layer)
```html
<!-- Position classes for large circles -->
<div class="absolute w-96 h-96 bg-gradient-to-br from-blue-500/30 to-blue-700/20 rounded-full -top-20 -left-20 animate-pulse"></div>
<div class="absolute w-64 h-64 bg-gradient-to-br from-red-500/40 to-orange-600/30 rounded-full top-40 right-10 animate-pulse" style="animation-delay: 0.5s"></div>
<div class="absolute w-80 h-80 bg-gradient-to-br from-yellow-400/35 to-amber-500/25 rounded-full bottom-20 left-1/4 animate-pulse" style="animation-delay: 1s"></div>
<div class="absolute w-48 h-48 bg-gradient-to-br from-green-500/45 to-emerald-600/35 rounded-full top-1/3 left-1/2 animate-pulse" style="animation-delay: 1.5s"></div>
<div class="absolute w-72 h-72 bg-gradient-to-br from-purple-500/30 to-violet-600/20 rounded-full -bottom-10 -right-10 animate-pulse" style="animation-delay: 2s"></div>
```

### Small Accent Circles
```html
<!-- Smaller animated circles for visual interest -->
<div class="absolute w-24 h-24 bg-gradient-to-br from-pink-500/50 to-rose-600/40 rounded-full top-20 left-1/3 animate-bounce" style="animation-delay: 0.8s; animation-duration: 3s"></div>
<div class="absolute w-16 h-16 bg-gradient-to-br from-indigo-500/60 to-blue-600/50 rounded-full bottom-1/3 right-1/4 animate-bounce" style="animation-delay: 1.2s; animation-duration: 2.5s"></div>
<div class="absolute w-20 h-20 bg-gradient-to-br from-teal-500/55 to-cyan-600/45 rounded-full top-2/3 left-20 animate-bounce" style="animation-delay: 1.8s; animation-duration: 3.5s"></div>
```

## Component Patterns

### Section Containers
```html
<!-- Standard section with backdrop blur -->
<div class="bg-white/10 backdrop-blur-sm rounded-3xl p-6 md:p-8 shadow-xl border border-gray-700/50 mb-8">
  <div class="max-w-4xl mx-auto">
    <!-- Section content -->
  </div>
</div>
```

### Card Components
```html
<!-- Feature cards -->
<div class="bg-gray-800/50 backdrop-blur-sm rounded-2xl p-6 border border-gray-600/30 hover:border-orange-400/50 transition-all group hover:bg-gray-700/50">
  <!-- Card content -->
</div>
```

### Input Elements
```html
<!-- Search inputs with Kandinsky styling -->
<input
  type="text"
  class="w-full p-4 pr-12 border-2 border-gray-600 rounded-full outline-none bg-gray-800/80 backdrop-blur-sm text-white placeholder:text-gray-400 focus:border-orange-400 focus:ring-4 focus:ring-orange-200/50 transition-all shadow-lg hover:shadow-xl transform hover:scale-[1.02]"
/>
```

### Button Styles
```html
<!-- Primary action button -->
<a class="flex-shrink-0 w-14 h-14 bg-gradient-to-br from-red-400 to-orange-500 hover:from-red-500 hover:to-orange-600 rounded-full flex items-center justify-center transition-all group shadow-lg hover:shadow-xl transform hover:scale-110 hover:rotate-12">
  <!-- Icon -->
</a>

<!-- Secondary button -->
<button class="px-6 py-3 bg-gradient-to-r from-orange-500 to-red-500 hover:from-orange-600 hover:to-red-600 text-white text-sm font-bold rounded-full transition-all transform hover:scale-105 hover:shadow-lg">
  Button Text
</button>
```

## Typography System

### Headings
```css
/* Main page title */
text-4xl md:text-5xl lg:text-6xl font-bold text-white mb-4

/* Section headings */
text-2xl md:text-3xl font-bold text-white mb-6

/* Subsection headings */
text-xl font-bold text-gray-100 mb-4

/* Card headings */
text-lg font-bold text-white mb-3
```

### Body Text
```css
/* Primary body text */
text-gray-300 text-base md:text-lg leading-relaxed

/* Secondary/muted text */
text-gray-400 text-sm md:text-base

/* Small text/captions */
text-gray-400 text-xs md:text-sm
```

## Animation System

### Circle Animations
```css
/* Pulse animation for background circles */
animate-pulse

/* Bounce animation for accent circles */
animate-bounce
animation-delay: 0.8s
animation-duration: 3s

/* Staggered delays for visual rhythm */
style="animation-delay: 0.5s"
style="animation-delay: 1s"
style="animation-delay: 1.5s"
style="animation-delay: 2s"
```

### Interactive Animations
```css
/* Hover scaling */
transform hover:scale-[1.02]     /* Subtle scale for inputs */
transform hover:scale-105        /* Medium scale for buttons */
transform hover:scale-110        /* Prominent scale for icons */

/* Rotation on hover */
hover:rotate-12                  /* Playful rotation for circular elements */

/* Shadow transitions */
shadow-lg hover:shadow-xl        /* Depth enhancement */
```

## Mobile-First Responsive Design

### Breakpoint Strategy
```css
/* Base (mobile): 0px and up */
px-4 py-8 text-base

/* MD (tablet): 768px and up */
md:px-6 md:py-12 md:text-lg

/* LG (desktop): 1024px and up */
lg:px-8 lg:py-16 lg:text-xl

/* XL (large desktop): 1280px and up */
xl:max-w-7xl xl:px-12
```

### Mobile Adaptations
```html
<!-- Responsive grid layouts -->
<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 md:gap-8">

<!-- Responsive flex layouts -->
<div class="flex flex-col md:flex-row items-center gap-4 md:gap-8">

<!-- Responsive text sizing -->
<h1 class="text-3xl md:text-4xl lg:text-5xl font-bold">

<!-- Responsive spacing -->
<div class="px-4 py-6 md:px-6 md:py-8 lg:px-8 lg:py-12">
```

## Header and Footer Integration

### Header Styling
```html
<header class="bg-gray-900/95 backdrop-blur-sm border-b border-gray-700/50 sticky top-0 z-50">
  <nav class="max-w-7xl mx-auto px-4 md:px-6">
    <!-- Navigation content with consistent colors -->
  </nav>
</header>
```

### Footer Styling
```html
<footer class="bg-gray-900/90 backdrop-blur-sm border-t border-gray-700/50 mt-auto">
  <div class="max-w-7xl mx-auto px-4 md:px-6 py-8 md:py-12">
    <!-- Footer content -->
  </div>
</footer>
```

## Implementation Guidelines

### Template Structure
1. **Wrapper**: Always start with overflow-hidden container
2. **Background Layer**: Floating circles with pointer-events-none
3. **Content Layer**: Relative z-10 positioning for all interactive content
4. **Responsive**: Mobile-first approach with progressive enhancement

### Color Application Rules
1. **High Contrast**: Ensure minimum 4.5:1 contrast ratio for text
2. **Transparency**: Use /10, /20, /30 opacity levels for layering
3. **Consistency**: Stick to the defined gradient combinations
4. **Accessibility**: Test with color blindness simulators

### Performance Considerations
1. **Backdrop Blur**: Use sparingly to maintain performance
2. **Animations**: Prefer CSS transforms over layout changes
3. **Z-index**: Maintain clear stacking context with background (0), content (10), modals (50+)

### File Organization
- Colors and gradients defined in this document
- Reusable component classes can be extracted to utility classes
- Animation delays and durations documented for consistency

## Extension Guidelines

When creating new pages or components:

1. **Start with the base wrapper pattern** (overflow-hidden container)
2. **Add appropriate floating circles** for the page size and content
3. **Use consistent section containers** with backdrop blur
4. **Apply the color palette** systematically
5. **Test mobile-first** responsive behavior
6. **Maintain animation consistency** with documented delays and durations

This design system ensures visual consistency while maintaining the sophisticated, playful aesthetic of Kandinsky's "Several Circles" across the entire application.