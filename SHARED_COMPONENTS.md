# Shared Components Implementation

This document describes the shared components and utilities that were extracted to reduce code duplication across the multi-page application.

## Overview

Instead of converting to a Single-Page Application (SPA), we've extracted common code into shared modules while maintaining the Multi-Page Application (MPA) architecture. This provides the benefits of code reuse without the complexity of a build system.

## Changes Made

### 1. ES Module Support

All HTML pages now load JavaScript as ES modules:

```html
<!-- Before -->
<script src="js/app.js"></script>

<!-- After -->
<script type="module" src="js/app.js"></script>
```

This enables the use of `import`/`export` statements for better code organization.

### 2. Shared Navigation Component

**File:** `web/js/shared/navigation.js`

Provides a reusable Vue component for the navigation bar that appears on all pages.

**Usage:**
```javascript
import { createNavigation } from './shared/navigation.js';

// Register the component
app.component('app-nav', createNavigation('viewer'));
```

**Parameters:**
- `activePage`: String indicating which page is active ('viewer', 'requests', 'settings', 'request-detail')

**Benefits:**
- Navigation markup now defined in one place instead of four
- Active page highlighting handled automatically
- Consistent navigation across all pages

### 3. Shared API Utilities

**File:** `web/js/shared/api.js`

Provides standardized HTTP request methods and utility functions.

**API Object:**
```javascript
import { API } from './shared/api.js';

// GET request
const data = await API.get('/api/containers');

// POST request
const result = await API.post('/api/save-trace', { data });

// PUT request
await API.put('/api/servers/123', { name: 'Updated' });

// DELETE request
await API.delete('/api/servers/123');
```

**Format Object:**
```javascript
import { Format } from './shared/api.js';

// Format dates
const dateStr = Format.date(new Date());

// Format JSON
const jsonStr = Format.json({ key: 'value' }, 2);

// Format duration
const duration = Format.duration(1234); // "1.23s"
```

**Storage Object:**
```javascript
import { Storage } from './shared/api.js';

// Get from localStorage
const value = Storage.get('key', defaultValue);

// Set in localStorage
Storage.set('key', value);

// Remove from localStorage
Storage.remove('key');
```

**Benefits:**
- Consistent error handling across all API calls
- Automatic JSON parsing
- Centralized logging of errors
- Reusable format utilities

### 4. Updated Utils Module

**File:** `web/js/utils.js`

Now exports `formatSQL` function as an ES module:

```javascript
export function formatSQL(sql) {
  // ... implementation
}
```

This allows it to be imported by other modules that need SQL formatting.

## File Structure

```
web/
├── js/
│   ├── shared/
│   │   ├── navigation.js    # Shared navigation component
│   │   └── api.js           # API utilities and helpers
│   ├── app.js               # Log Viewer page (uses shared components)
│   ├── requests.js          # Request Manager page (uses shared components)
│   ├── settings.js          # Settings page (uses shared components)
│   ├── request-detail.js    # Request Detail page (uses shared components)
│   └── utils.js             # SQL formatting utility (now exports)
├── index.html
├── requests.html
├── settings.html
└── request-detail.html
```

## Migration Guide

To use the shared components in a new page:

1. Update the HTML to load scripts as modules:
```html
<script type="module" src="js/new-page.js"></script>
```

2. Import the shared components:
```javascript
import { createNavigation } from './shared/navigation.js';
import { API, Format, Storage } from './shared/api.js';
```

3. Register the navigation component:
```javascript
app.component('app-nav', createNavigation('page-name'));
```

4. Use `<app-nav></app-nav>` in your template instead of hard-coded navigation

5. Use `API.get()`, `API.post()`, etc. instead of raw `fetch()` calls

## Benefits

### Before (Duplicated Code)
- Navigation defined in 4 places (12 lines × 4 = 48 lines)
- Fetch calls with custom error handling in each file
- Inconsistent API patterns

### After (Shared Components)
- Navigation defined once (shared/navigation.js)
- Standardized API calls with consistent error handling
- ~200 lines of reusable code in shared modules
- Easier to maintain and update

## Testing

All pages have been tested to ensure:
- ✅ Navigation renders correctly on all pages
- ✅ Active page is highlighted correctly
- ✅ Navigation links work correctly
- ✅ ES module imports work in the browser
- ✅ API utilities are available to all pages

## Future Improvements

Potential additions to shared modules:

1. **Shared Modal Components** - Extract common modal patterns
2. **Form Validation** - Shared form validation utilities
3. **Error Display** - Consistent error message display
4. **Loading States** - Reusable loading indicators

## Notes

- No build process required - modules work natively in modern browsers
- Backward compatible with existing functionality
- Can still deploy as simple static files
- Easy for contributors to understand and use
