## Single Purpose Description

"Pensive is a web bookmarking and content indexing extension that allows users to save web pages they read for later searchability. Users can click the extension icon to extract and save page content (title, text, and clean HTML) to their personal Pensive account for building a searchable knowledge library."
Permission Justifications

### activeTab Permission

"The activeTab permission is required to access the current webpage's URL and title when the user clicks the extension icon to save a page. This permission allows the extension to identify which page the user wants to bookmark and retrieve basic page metadata (URL, title) for saving to their personal library."

### Host Permission (localhost:8000/, getpensive.com/)

"Host permissions are required to communicate with the user's Pensive backend service. The extension needs to authenticate with the user's account, check if pages are already bookmarked, and send extracted page content to be saved. Communication only occurs with the official Pensive service endpoints (getpensive.com for production, localhost for development)."

### Remote Code Use (Readability.js)
"The extension uses the Mozilla Readability.js library (included in the extension package, not downloaded remotely) to extract clean, readable content from web pages. This library removes ads, navigation elements, and other page noise to extract only the main article content, improving the quality of saved content for later searching."

### Scripting Permission
"The scripting permission is required to inject content extraction scripts into web pages when users save them. These scripts use the Readability.js library to analyze the page structure and extract the main article content (text and clean HTML) while removing advertisements, navigation menus, and other non-content elements."

### Storage Permission
"The storage permission is required to save the user's authentication token and backend server endpoint configuration locally. This allows the extension to remember the user's login status and communicate with their Pensive account without requiring re-authentication on every use."

### Data Usage Compliance Statement
"This extension only processes and transmits data when explicitly requested by the user (clicking to save a page). All extracted page content is sent directly to the user's private Pensive account and is not shared with third parties. No user data is collected, stored, or transmitted for advertising, analytics, or any purpose other than the core bookmarking functionality."
These justifications align with your project's core purpose of helping users build a searchable personal knowledge library from web content they choose to save.