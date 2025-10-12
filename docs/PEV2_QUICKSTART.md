# PEV2 Integration - Quick Start Guide

## What Was Integrated

PEV2 (PostgreSQL Explain Visualizer 2) from https://github.com/dalibo/pev2 has been successfully integrated into the Docker Log Viewer to provide advanced visualization of PostgreSQL EXPLAIN plans.

## Quick Test

1. **Start the viewer:**
   ```bash
   cd /home/runner/work/docker-log-viewer/docker-log-viewer
   ./docker-log-viewer
   ```

2. **Test the integration:**
   - Open http://localhost:9000/test-pev2.html
   - Verify both Vue 3 and PEV2 libraries loaded successfully
   - Click "Load Sample EXPLAIN Plan" or "Load Complex Plan"
   - The PEV2 component will display an interactive visualization

3. **Use in production:**
   - Configure PostgreSQL connection:
     ```bash
     export DATABASE_URL="postgresql://user:pass@localhost:5432/db"
     ./docker-log-viewer
     ```
   - Open http://localhost:9000
   - Filter logs by trace/request/span ID to activate SQL Analyzer
   - Click "Run EXPLAIN" on any query
   - View the PEV2 visualization in the modal

## Key Features

### Before (Custom Formatter)
- Simple text-based tree display
- Basic cost and row information
- Limited interactivity
- No advanced metrics

### After (PEV2)
- **Interactive tree visualization** with expand/collapse
- **Visual cost indicators** with color coding
- **Detailed node information** on hover/click
- **Performance metrics** (actual timing, row counts)
- **SQL syntax highlighting**
- **Professional UI** used by PostgreSQL experts

## Architecture

```
┌─────────────────────────────────────────┐
│  Docker Log Viewer Frontend             │
│                                         │
│  ┌──────────────┐  ┌─────────────┐    │
│  │   Vue 3      │  │   PEV2      │    │
│  │  (156 KB)    │  │  (499 KB)   │    │
│  └──────────────┘  └─────────────┘    │
│           ▲              ▲             │
│           │              │             │
│  ┌────────┴──────────────┴──────────┐ │
│  │     app.js (DockerLogParser)    │ │
│  │  - initPEV2()                    │ │
│  │  - showExplainResult()           │ │
│  └──────────────────────────────────┘ │
└─────────────────────────────────────────┘
                   │
                   │ /api/explain
                   ▼
┌─────────────────────────────────────────┐
│  Go Backend (cmd/viewer/main.go)        │
│  - handleExplain()                      │
│  - Returns JSON EXPLAIN plans           │
└─────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  PostgreSQL Database                    │
│  - EXPLAIN (ANALYZE, FORMAT JSON)       │
└─────────────────────────────────────────┘
```

## Files Added/Modified

### Added (961 KB total)
- `web/pev2.umd.js` (499 KB) - PEV2 library
- `web/pev2.css` (15 KB) - PEV2 styles
- `web/vue.global.prod.js` (156 KB) - Vue 3 runtime
- `web/bootstrap.min.css` (227 KB) - Bootstrap CSS
- `web/test-pev2.html` (10 KB) - Test page
- `docs/PEV2_INTEGRATION.md` (7 KB) - Documentation

### Modified
- `web/index.html` - Added libraries, updated EXPLAIN modal
- `web/app.js` - Replaced custom formatter with PEV2
- `web/style.css` - Added dark theme integration
- `README.md` - Updated features
- `AGENTS.md` - Updated architecture

## Benefits

1. **Better User Experience**
   - Professional visualization tool
   - Interactive exploration of query plans
   - Easier to spot performance issues

2. **Industry Standard**
   - Developed and maintained by Dalibo (PostgreSQL experts)
   - Used by many PostgreSQL professionals
   - Regular updates and improvements

3. **Feature Complete**
   - All EXPLAIN plan information displayed
   - Advanced metrics and statistics
   - Plan comparison capabilities (future enhancement)

4. **Offline Operation**
   - All dependencies bundled locally
   - No CDN required
   - Works in air-gapped environments

## Testing Results

✅ All Go tests pass (`go test ./...`)
✅ Build successful (`go build -o docker-log-viewer cmd/viewer/main.go`)
✅ Vue 3 loads correctly (156 KB)
✅ PEV2 loads correctly (499 KB)
✅ Component initialization successful
✅ Dark theme CSS applied
✅ Server starts and serves all files

## Next Steps

1. **Testing with Real Data**
   - Connect to a PostgreSQL database
   - Run actual EXPLAIN queries
   - Verify visualization with complex plans

2. **Potential Enhancements**
   - Plan comparison feature
   - Plan history storage
   - Export plans as images
   - Share plans via links

3. **Documentation**
   - See `docs/PEV2_INTEGRATION.md` for detailed documentation
   - See `docs/SQL_EXPLAIN.md` for EXPLAIN feature setup
   - See `README.md` for general usage

## Troubleshooting

### Libraries not loading
- Check browser console for errors
- Verify files exist in `web/` directory
- Ensure server is serving static files from `./web`

### PEV2 not displaying
- Check that `/api/explain` returns valid JSON
- Verify PostgreSQL connection (DATABASE_URL)
- Check browser console for Vue/PEV2 errors

### Dark theme issues
- Ensure `style.css` loads after `pev2.css`
- CSS overrides use `!important` for specificity

## References

- PEV2 GitHub: https://github.com/dalibo/pev2
- PEV2 Demo: https://explain.dalibo.com
- Vue 3 Docs: https://vuejs.org
- PostgreSQL EXPLAIN: https://www.postgresql.org/docs/current/sql-explain.html
