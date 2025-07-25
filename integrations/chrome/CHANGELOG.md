# Changelog

> **Changelog Requirements**: Each version should includes a brief feature summary paragraph, followed by technical bullet points detailing implementation changes. Keep entries concise but technically accurate.

## Version 0.2.0 Beta - 25 July

Streamlined extension configuration and improved authentication flow. Removed user-configurable endpoints and enhanced token management with proper server-side cleanup.

- **Removed endpoint configuration**: Eliminated user input for server endpoint in options.html and simplified options.js to use fixed endpoint based on devMode flag
- **Added website integration**: Integrated getpensive.com link in options page for user onboarding
- **Updated UI elements**: Changed header icon from edit/pen SVG to bookmark SVG, condensed usage instructions from 4 steps to 3
- **Enhanced token validation**: Modified validateToken() to use authenticated `/api/v1/ping` endpoint instead of public `/api/ping`
- **Implemented proper sign out**: Added DELETE request to `/api/v1/tokens/current` before local token removal, with graceful fallback on server errors

## Verion 0.1.1 Beta - 24 July

Initia release
