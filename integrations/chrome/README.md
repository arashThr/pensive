# Pensive Chrome extension

## Development:
- Set `devMode` to true in `options.js`
- Add localhost to these fields:
```
  "optional_host_permissions": [
    "http://localhost:8000/*",
    "https://getpensive.com/*"
  ],
...
    "content_scripts": [
      {
        "matches": [
          "http://localhost:8000/extension/auth",
          "https://getpensive.com/extension/auth"
        ],
        "js": ["content.js"]
      }
    ],
    "web_accessible_resources": [{
      "resources": ["options.js", "Readability.js", "Readability-readerable.js"],
      "matches": [
        "http://localhost:8000/*",
        "https://getpensive.com/*"
      ]
    }]
```