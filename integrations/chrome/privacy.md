## Single Purpose Description

"Pensive is a web bookmarking and content indexing extension that allows users to save web pages they read for later searchability. Users can click the extension icon to extract and save page content using one of three configurable extraction methods to their personal Pensive account for building a searchable knowledge library."

## Content Extraction Methods

The extension offers three content extraction methods:

**Server-Side Extraction (Most Secure):** Extension sends only the page URL to Pensive servers, which fetch and process the content. Most secure as no page content leaves your browser.

**Smart Article Extraction (Recommended):** Extension uses Mozilla Readability locally to extract clean article content, then sends only the main text. Balances security and quality.

**Full Page Extraction (Most Complete):** Extension captures entire page content locally and sends processed HTML. Most accurate but may include sensitive information from logged-in pages. Users should avoid this method for pages containing personal information, emails, or sensitive data.

## Permission Justifications

### activeTab Permission

"The activeTab permission is required to access the current webpage's URL, title, and content when the user clicks the extension icon to save a page. This permission allows the extension to identify which page the user wants to bookmark and extract content using the selected extraction method."

### Host Permission (getpensive.com/)

"Host permissions are required to communicate with the user's Pensive backend service. The extension needs to authenticate with the user's account, check if pages are already bookmarked, and send extracted page content to be saved. Communication only occurs with the official Pensive service endpoints."

### Remote Code Use (Readability.js)
"The extension uses the Mozilla Readability.js library (included in the extension package, not downloaded remotely) to extract clean, readable content from web pages when using Smart Article or Full Page extraction methods. This library removes ads, navigation elements, and other page noise to extract only the main article content, improving the quality of saved content for later searching."

### Scripting Permission
"The scripting permission is required to inject content extraction scripts into web pages when users save them using client-side extraction methods. These scripts use the Readability.js library to analyze the page structure and extract the main article content (text and clean HTML) while removing advertisements, navigation menus, and other non-content elements."

### Storage Permission
"The storage permission is required to save the user's authentication token, selected extraction method, and site-specific preferences locally. This allows the extension to remember the user's login status, extraction preferences, and provide a consistent experience without requiring re-authentication on every use."

### Data Usage Compliance Statement
"This extension only processes and transmits data when explicitly requested by the user (clicking to save a page). The type and amount of data transmitted depends on the selected extraction method. All extracted page content is sent directly to the user's private Pensive account and is not shared with third parties. No user data is collected, stored, or transmitted for advertising, analytics, or any purpose other than the core bookmarking functionality."

### User Responsibility for Shared Content
"When using client-side extraction methods (Smart Article or Full Page), users are responsible for ensuring they do not share content they wish to keep private. While Pensive does not use, analyze, or share user content for any purpose other than providing the bookmarking service, users should exercise caution when using Full Page Extraction on pages containing personal information, emails, private messages, or other sensitive data. This is particularly important for logged-in pages or sites with user-specific content."