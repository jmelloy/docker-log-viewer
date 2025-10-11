# PEV2 Integration Documentation

## Overview

This document describes the integration of PEV2 (PostgreSQL Explain Visualizer 2) into the Docker Log Viewer application. PEV2 provides an advanced, interactive visualization of PostgreSQL EXPLAIN plans.

## What is PEV2?

PEV2 is a VueJS component developed by Dalibo that provides a graphical visualization of PostgreSQL execution plans. It offers:

- **Interactive tree visualization** of query execution plans
- **Cost analysis** with visual indicators
- **Performance metrics** including actual timing and row counts
- **Detailed node information** with expandable sections
- **SQL syntax highlighting**
- **Plan comparison** capabilities

Project: https://github.com/dalibo/pev2

## Integration Details

### Files Added

1. **web/pev2.umd.js** (499 KB) - PEV2 UMD bundle
2. **web/pev2.css** (15 KB) - PEV2 styles

### Files Modified

1. **web/index.html**
   - Added Bootstrap CSS dependency (required by PEV2)
   - Added PEV2 CSS
   - Added Vue 3 from CDN
   - Added PEV2 UMD bundle
   - Modified EXPLAIN modal to use PEV2 component

2. **web/app.js**
   - Added `pev2App` property to DockerLogParser class
   - Added `initPEV2()` method to initialize Vue app with PEV2 component
   - Modified `showExplainResult()` to use PEV2 instead of custom formatter
   - Removed obsolete methods: `formatExplainPlan()`, `formatSQL()`, `highlightSQL()`

3. **web/style.css**
   - Increased EXPLAIN modal max-width to 1400px
   - Added PEV2-specific styling for dark theme compatibility
   - Added CSS overrides for Bootstrap components in PEV2

## How It Works

### 1. Initialization

When the DockerLogParser app initializes, it creates a Vue 3 application instance for PEV2:

```javascript
initPEV2() {
  const { createApp } = Vue;
  this.pev2App = createApp({
    data() {
      return {
        planSource: '',
        planQuery: ''
      }
    },
    methods: {
      updatePlan(plan, query) {
        this.planSource = plan;
        this.planQuery = query;
      }
    }
  });
  this.pev2App.component('pev2', pev2.Plan);
  this.pev2App.mount('#pev2App');
}
```

### 2. Displaying EXPLAIN Plans

When a user clicks "Run EXPLAIN" on a SQL query:

1. The query and variables are sent to `/api/explain` endpoint
2. Backend executes `EXPLAIN (ANALYZE, FORMAT JSON)` on PostgreSQL
3. Backend returns JSON response with query plan
4. Frontend converts the plan to a JSON string
5. Frontend updates PEV2 component with the plan data
6. PEV2 renders an interactive visualization

```javascript
showExplainResult(result) {
  if (result.error) {
    // Show error message
    const pev2El = document.getElementById('pev2App');
    pev2El.innerHTML = `<div class="alert alert-danger m-3">${this.escapeHtml(result.error)}</div>`;
  } else {
    // Convert and update PEV2
    let planText = JSON.stringify(result.queryPlan, null, 2);
    this.pev2App._instance.proxy.updatePlan(planText, result.query || '');
  }
  document.getElementById("explainModal").classList.remove("hidden");
}
```

### 3. Backend Support

The backend (Go) code remains unchanged because it already returns EXPLAIN plans in the correct format:

- Endpoint: `POST /api/explain`
- Request: `{ "query": "SELECT...", "variables": {"1": "value"} }`
- Response: `{ "queryPlan": [...], "query": "...", "error": "..." }`

The `queryPlan` field contains the JSON array returned by PostgreSQL's `EXPLAIN (FORMAT JSON)`.

## Features Enabled

With PEV2 integration, users can now:

1. **Visualize Execution Plans** - See query plans as interactive tree diagrams
2. **Analyze Performance** - View cost estimates, actual timing, and row counts
3. **Inspect Node Details** - Click on nodes to see detailed information
4. **Understand Query Flow** - Follow the execution path visually
5. **Identify Bottlenecks** - Spot expensive operations quickly
6. **Compare Alternatives** - Understand why PostgreSQL chose a specific plan

## Usage

1. Start the Docker Log Viewer: `./docker-log-viewer`
2. Filter logs by trace/request/span ID to activate SQL Analyzer
3. Click "Run EXPLAIN" on any SQL query in the analyzer
4. Interact with the PEV2 visualization:
   - Expand/collapse nodes
   - Hover for tooltips
   - Click for detailed information
   - Zoom and pan the diagram

## Dark Theme Integration

Custom CSS has been added to ensure PEV2 looks good with the Docker Log Viewer's dark theme:

```css
#pev2App .card {
    background-color: #161b22 !important;
    border-color: #30363d !important;
    color: #c9d1d9 !important;
}

#pev2App .form-control,
#pev2App .form-select {
    background-color: #0d1117 !important;
    border-color: #30363d !important;
    color: #c9d1d9 !important;
}
```

## Benefits Over Previous Implementation

The previous custom formatter displayed EXPLAIN plans as a simple text tree. PEV2 provides:

- **Better Visualization** - Interactive tree with collapsible nodes
- **More Information** - Detailed metrics and statistics per node
- **Better UX** - Hover tooltips, clickable nodes, zoom/pan
- **Industry Standard** - Well-maintained, widely-used tool
- **Feature Complete** - Plan statistics, node details, SQL formatting
- **Professional** - Used by PostgreSQL experts at Dalibo

## Troubleshooting

### PEV2 doesn't display

**Problem:** The EXPLAIN modal opens but shows no visualization.

**Solution:** Check browser console for errors. Ensure:
- Vue 3 CDN is accessible
- pev2.umd.js loaded successfully
- No JavaScript errors during initialization

### Dark theme issues

**Problem:** PEV2 components appear with light background.

**Solution:** The custom CSS overrides should handle this. If not, check:
- style.css is loaded after pev2.css
- CSS specificity is sufficient (use `!important` if needed)

### Database connection issues

**Problem:** "Database connection not configured" error.

**Solution:** Set the `DATABASE_URL` environment variable:
```bash
export DATABASE_URL="postgresql://user:pass@localhost:5432/db"
./docker-log-viewer
```

## Future Enhancements

Potential improvements to the PEV2 integration:

1. **Plan Comparison** - Allow comparing execution plans for different query variations
2. **Plan History** - Store and retrieve historical EXPLAIN plans
3. **Export Plans** - Download plans as JSON or images
4. **Plan Sharing** - Generate shareable links (like explain.dalibo.com)
5. **Custom Highlighting** - Highlight expensive operations automatically
6. **Plan Diff** - Visual diff between before/after query optimization

## References

- PEV2 GitHub: https://github.com/dalibo/pev2
- PEV2 Live Demo: https://explain.dalibo.com
- PostgreSQL EXPLAIN: https://www.postgresql.org/docs/current/sql-explain.html
- Vue 3 Documentation: https://vuejs.org/guide/introduction.html
