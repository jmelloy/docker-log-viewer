# SQL Query Export Feature - Implementation Summary

## Overview
Added markdown and Notion export capabilities for SQL query analysis reports in the Docker Log Viewer.

## Features Implemented

### 1. Markdown Export (Client-side)
**Location:** `web/src/views/SqlDetailView.vue`

**UI Changes:**
- Added "📄 Export Markdown" button in the SQL Query Detail page header
- Button appears next to the "← Back" button
- No additional configuration required

**Functionality:**
- Generates comprehensive markdown document including:
  - Report metadata (generation date)
  - Query information (operation, table, execution stats)
  - SQL query (formatted with newlines)
  - Normalized query (parameterized version)
  - EXPLAIN plan (text format from JSON)
  - Index recommendations (if available)
  - Related executions table
- Downloads as `.md` file with timestamp
- Filename format: `sql-query-{hash}-{timestamp}.md`

**Example Markdown Output:**
```markdown
# SQL Query Analysis Report

**Generated:** Tue, 19 Nov 2024 14:30:00 GMT

**Request ID:** req-abc-123
**Execution Date:** Tue, 19 Nov 2024 13:45:00 GMT

## Query Information

- **Operation:** SELECT
- **Table:** users
- **Total Executions:** 42
- **Average Duration:** 15.50ms
- **Min Duration:** 5.20ms
- **Max Duration:** 35.80ms

## SQL Query

```sql
SELECT users.id, users.name, users.email
FROM users
WHERE users.active = true
ORDER BY users.created_at DESC
LIMIT 100
```

## EXPLAIN Plan

```
{
  "Plan": {
    "Node Type": "Index Scan",
    "Total Cost": 100.50,
    ...
  }
}
```

## Related Executions

| Request ID | Display Name | Status | Duration | Executed At |
|------------|--------------|--------|----------|-------------|
| req-abc-123 | GetUsers | 200 | 15.50ms | ... |
```

### 2. Notion Export (Server-side)
**Location:** `cmd/viewer/main.go`

**UI Changes:**
- Added "📘 Export to Notion" button next to Markdown export button
- Shows success message with link to created page
- Shows error message if configuration missing

**API Endpoint:**
```
POST /api/sql/{queryHash}/export-notion
```

**Response:**
```json
{
  "url": "https://www.notion.so/page-id",
  "message": "Successfully exported to Notion"
}
```

**Configuration Required:**
Environment variables must be set:
- `NOTION_API_KEY`: Your Notion integration token
- `NOTION_DATABASE_ID`: Target database ID

**Notion Page Structure:**
1. **Title:** "SQL Query: {operation} on {table}"
2. **Query Information** (bulleted list)
   - Operation, Table, Executions, Durations, Request ID, Date
3. **SQL Query** (code block with SQL syntax highlighting)
4. **Normalized Query** (code block)
5. **EXPLAIN Plan** (code block with JSON formatting)

**Features:**
- Text truncation to fit Notion API limits (2000 chars per block)
- Proper JSON formatting for EXPLAIN plans
- SQL syntax highlighting in code blocks
- Error handling with descriptive messages

## Technical Implementation

### Frontend (Vue.js)

**New Methods in SqlDetailView.vue:**

1. `generateMarkdown()`: Creates markdown string from SQL detail
2. `formatExplainPlanAsText()`: Converts JSON explain plan to readable text
3. `exportAsMarkdown()`: Triggers browser download
4. `exportToNotion()`: Calls backend API

**Code Quality:**
- TypeScript-safe
- Consistent with existing Vue patterns
- Proper error handling
- User-friendly alerts

### Backend (Go)

**New Functions in cmd/viewer/main.go:**

1. `handleSQLNotionExport()`: HTTP handler for Notion export
2. `createNotionPage()`: Notion API integration
3. `formatSQLForDisplay()`: SQL formatting with newlines
4. `formatExplainPlanForNotion()`: EXPLAIN plan formatting
5. `truncateText()`: Ensures text fits API limits

**Security:**
- Environment variable validation
- HTTP timeout (30s)
- Input sanitization
- Error message safety (no secrets leaked)

### Tests

**Unit Tests (export_test.go):**
- `TestFormatSQLForDisplay`: SQL formatting
- `TestTruncateText`: Text truncation
- `TestFormatExplainPlanForNotion`: EXPLAIN plan formatting
- `TestCreateNotionPagePayload`: Notion payload generation

**Integration Test (markdown_test.go):**
- `TestMarkdownExportGeneration`: Complete markdown flow
- Validates all sections present
- Checks formatting correctness
- Verifies table structure

**Test Results:** ✅ 13/13 passing

## Documentation

**NOTION_EXPORT.md** includes:
- Step-by-step Notion integration setup
- Database configuration guide
- Usage instructions
- Troubleshooting tips
- API documentation

## File Changes

```
NOTION_EXPORT.md                | 137 lines (new)
cmd/viewer/export_test.go       | 172 lines (new)
cmd/viewer/main.go              | 367 lines (added)
cmd/viewer/markdown_test.go     | 135 lines (new)
web/src/views/SqlDetailView.vue | 195 lines (added)
```

**Total:** ~1006 lines added

## Build & Test Status

✅ Frontend builds successfully (no TypeScript errors)
✅ Backend builds successfully (no Go errors)
✅ All new tests passing (13/13)
✅ No security vulnerabilities (CodeQL)
✅ No breaking changes to existing code

## Usage Examples

### Markdown Export
1. Navigate to SQL query detail page (`/sql/{hash}`)
2. Click "📄 Export Markdown" button
3. File downloads automatically to default location
4. Open in any markdown editor/viewer

### Notion Export
1. Set environment variables:
   ```bash
   export NOTION_API_KEY="secret_xxx..."
   export NOTION_DATABASE_ID="abc123..."
   ```
2. Restart application
3. Navigate to SQL query detail page
4. Click "📘 Export to Notion" button
5. Success message shows with link
6. Click link to open in Notion

## Benefits

1. **Easy Sharing:** Export and share SQL analysis with team members
2. **Documentation:** Keep records of query performance over time
3. **Notion Integration:** Centralize documentation in team workspace
4. **No Lock-in:** Markdown format is portable and version-control friendly
5. **Automated:** No manual copy-paste required

## Future Enhancements (Not in this PR)

Potential improvements:
- Export multiple queries at once
- Custom Notion templates
- Export to other platforms (Confluence, Slack, etc.)
- Scheduled exports
- Query comparison exports
- PDF export option

## Notes

- Markdown export works without any configuration
- Notion export is optional and requires setup
- Both exports use the same data source
- No changes to database schema required
- Compatible with existing Docker Log Viewer functionality
