# Web Folder Structure

## Overview
Reorganized structure with shared components, unified templates, and consolidated styles.

## Directory Layout

```
web/
├── index.html              # Log viewer app
├── requests.html           # Request manager app
├── css/
│   └── styles.css          # Consolidated styles (merged style.css + requests.css)
├── js/
│   ├── app.js              # Log viewer Vue app
│   └── requests.js         # Request manager logic
└── lib/                    # Third-party libraries (optional local copies)
    ├── vue.global.prod.js
    ├── pev2.umd.js
    └── pev2.css
```

## Changes Made

1. **Consolidated CSS**: Merged `style.css` and `requests.css` into single `css/styles.css`
2. **Organized JS**: Moved all JavaScript to `js/` directory
3. **Removed Duplicates**: 
   - Removed `bootstrap.min.css` (using CDN)
   - Removed `test-pev2.html` (test file)
4. **Library Folder**: Created `lib/` for optional local library copies

## Notes

- `index.html` uses Vue 3 with minimal template (app rendered by Vue)
- `requests.html` uses vanilla JS with full HTML structure (DOM manipulation)

## Usage

- Both apps use CDN for Vue and PEV2 by default
- Local copies in `lib/` can be used as fallback
- All styling is in one file for easier maintenance
- Shared styles for modals, buttons, forms, etc.
