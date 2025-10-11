# Implementation Summary: SQLite Store & GraphQL Request Manager

## Overview

This implementation adds a complete GraphQL/API request management system with SQLite-backed storage, Docker log integration, and SQL query analysis capabilities. The system enables before/after performance analysis by tracking request executions over time.

## What Was Built

### 1. Database Layer (`pkg/store`)

**Files Created:**
- `pkg/store/schema.go` - SQLite database schema
- `pkg/store/store.go` - Complete CRUD operations and SQL analysis
- `pkg/store/store_test.go` - Comprehensive test coverage

**Database Schema:**
```
requests          - Saved GraphQL/API request templates
  ├── executions      - Each execution of a request
  │   ├── execution_logs  - Docker logs for each execution
  │   └── sql_queries     - Extracted SQL queries with metrics
```

**Features:**
- Foreign key constraints with cascade deletes
- Automatic SQL query normalization
- N+1 detection (queries executed >5 times)
- Performance metrics aggregation
- Full-text search on logs

### 2. CLI Tool (`cmd/graphql-tester`)

**File Created:**
- `cmd/graphql-tester/main.go` - Command-line interface

**Capabilities:**
```bash
# Save requests
./graphql-tester -url URL -data FILE [-name NAME]

# Execute immediately
./graphql-tester -url URL -data FILE -execute

# List saved requests
./graphql-tester -list

# Delete requests
./graphql-tester -delete ID

# With authentication
./graphql-tester -url URL -data FILE -token TOKEN -dev-id ID
```

**Integration:**
- Docker log streaming from all containers
- Automatic request ID generation
- Log collection with configurable timeout
- SQL query extraction from logs
- Same log matching logic as compare tool

### 3. Web UI

**Files Created:**
- `web/requests.html` - Request management interface
- `web/requests.css` - Styling for the UI
- `web/requests.js` - Frontend application logic

**UI Features:**

**Request Library:**
- List all saved requests
- Create new requests with form validation
- Delete requests with confirmation
- Filter and search capabilities

**Request Details:**
- View request configuration
- See execution history
- One-click request execution
- Execution list sorted by date

**Execution Analysis:**
- HTTP status and duration
- Full response body with JSON formatting
- Captured Docker logs with timestamps
- SQL query breakdown with metrics
- N+1 detection warnings
- Tables accessed summary

### 4. API Endpoints (Added to `cmd/viewer`)

**New Endpoints:**
```
GET    /api/requests              - List all requests
POST   /api/requests              - Create request
GET    /api/requests/:id          - Get request details
DELETE /api/requests/:id          - Delete request
POST   /api/requests/:id/execute  - Execute request (async)

GET    /api/executions?request_id=:id  - List executions
GET    /api/executions/:id             - Get execution detail
```

**Enhanced viewer:**
- Added Store integration
- Background request execution
- Log collection for executions
- SQL query extraction and storage

### 5. Documentation

**Files Created/Updated:**
- `docs/GRAPHQL_MANAGER.md` - Complete feature documentation
- `README.md` - Updated with new tool info
- `example-graphql-manager.sh` - Usage examples
- `build.sh` - Added graphql-tester to build

## Technical Highlights

### SQLite Schema Design
- **Normalized structure** with proper foreign keys
- **Cascade deletes** for data integrity
- **Indexes** on common query patterns
- **JSON storage** for flexible request data

### Log Collection Strategy
- Reuses existing Docker streaming infrastructure
- Matches logs by `request_id` field variations
- Configurable timeout (default 10s)
- Stores full log context for analysis

### SQL Query Extraction
- Detects `[sql]:` markers in logs
- Extracts duration, table, operation, rows
- Normalizes queries for grouping
- Calculates aggregate metrics

### Before/After Analysis Workflow
1. Execute baseline request
2. Make code changes
3. Execute same request again
4. Compare in Web UI:
   - Query counts and durations
   - Tables accessed
   - N+1 patterns
   - Response times

## Testing

All packages have test coverage:
```
✓ pkg/store/store_test.go - Full CRUD operations
✓ pkg/logs/*_test.go - Existing log parsing tests
✓ pkg/sqlexplain/*_test.go - SQL explain tests
```

Tested scenarios:
- Request creation and retrieval
- Execution tracking
- Log storage and retrieval
- SQL query extraction
- Foreign key cascades
- JSON parsing and validation

## Usage Examples

### Import GraphQL Operations
```bash
for file in graphql-operations-unique/*.json; do
  name=$(basename "$file" .json)
  ./graphql-tester -url "https://api.example.com/graphql" \
                   -data "$file" \
                   -name "$name"
done
```

### Execute and Analyze
```bash
# Execute once
./graphql-tester -url "https://api.staging.com/graphql" \
                 -data operations/FetchUsers.json \
                 -execute

# View results in Web UI
open http://localhost:9000/requests.html
```

### Before/After Comparison
1. Execute baseline
2. Deploy changes
3. Execute again
4. Compare executions side-by-side

## File Structure

```
cmd/
  graphql-tester/
    main.go                 - CLI tool
  viewer/
    main.go                 - Enhanced with request endpoints
pkg/
  store/
    schema.go               - Database schema
    store.go                - CRUD operations
    store_test.go           - Tests
web/
  requests.html             - Request manager UI
  requests.css              - Styling
  requests.js               - Frontend logic
  index.html                - Updated with navigation
docs/
  GRAPHQL_MANAGER.md        - Full documentation
example-graphql-manager.sh  - Usage examples
```

## Key Design Decisions

1. **SQLite for Storage**: Lightweight, file-based, no additional services
2. **Same Log Matching**: Reused compare tool's request ID matching
3. **Async Execution**: Requests execute in background, don't block UI
4. **JSON Storage**: Flexible request data format
5. **Cascade Deletes**: Automatic cleanup of related data
6. **Web + CLI**: Both interfaces for different use cases

## Integration Points

- **Docker API**: Container log streaming
- **Existing Log Parser**: Reuses parsing logic
- **SQL Analysis**: Same extraction as compare tool
- **Web Server**: Extends viewer with new endpoints

## Future Enhancement Opportunities

- Compare two executions side-by-side in UI
- Export/import request collections
- Request templates with variable substitution
- Scheduled execution
- Alert thresholds for performance
- GraphQL schema validation
- Response diff visualization

## Performance Considerations

- **Database**: Indexed for common queries
- **Log Collection**: Timeout prevents indefinite waiting
- **Memory**: Limited to 10,000 logs per execution
- **Storage**: Cascade deletes prevent bloat
- **UI**: Lazy loading of execution details

## Compliance with Requirements

✓ SQLite store for requests, responses, logs
✓ Web page for managing requests
✓ Same arguments as compare script
✓ Request tracking and history
✓ Log entry association
✓ Before/after analysis capability
✓ Uses graphql-operations as test samples

## Commands

```bash
# Build
go build -o graphql-tester ./cmd/graphql-tester
go build -o docker-log-viewer ./cmd/viewer

# Test
go test ./pkg/store -v

# Run
./docker-log-viewer  # Web UI at :9000
./graphql-tester -list
```

## Summary

This implementation provides a complete request management system that:
1. Stores GraphQL/API requests in SQLite
2. Executes requests with full log capture
3. Analyzes SQL query performance
4. Enables before/after comparisons
5. Provides both CLI and Web interfaces
6. Integrates seamlessly with existing tools
