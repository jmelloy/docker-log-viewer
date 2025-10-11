# URL Comparison Tool - Implementation Summary

## What was built

A command-line tool that compares two API endpoints (GraphQL or JSON) by:
1. Posting the same request to both URLs
2. Capturing the X-Request-Id from each response
3. Collecting Docker logs for each request
4. Analyzing SQL queries from the logs
5. Generating a side-by-side HTML comparison report

## Files created/modified

### New files:
- `cmd/compare-tool/main.go` - Main comparison tool implementation
- `pkg/logs/docker.go` - Docker client library (shared)
- `pkg/logs/parser.go` - Log parsing library (shared)
- `COMPARE-TOOL.md` - Detailed documentation
- `example-usage.sh` - Example usage script
- `generate-sample-output.sh` - Sample output generator
- `sample-request.json` - Sample GraphQL request
- `sample-query.json` - Sample GraphQL query

### Modified files:
- `main.go` - Updated to use pkg/logs package
- `README.md` - Added comparison tool section
- `AGENTS.md` - Updated file structure
- `.gitignore` - Added generated files

## Key features

1. **HTTP Request Handling**
   - Posts JSON/GraphQL to two URLs
   - Captures response headers (X-Request-Id)
   - Handles both JSON and GraphQL payloads

2. **Log Collection**
   - Monitors all running Docker containers
   - Filters logs by request_id field
   - Configurable timeout for log collection

3. **SQL Analysis**
   - Extracts SQL queries from logs
   - Normalizes queries for grouping
   - Calculates duration statistics
   - Detects N+1 query issues
   - Identifies slow queries

4. **HTML Report Generation**
   - Side-by-side comparison
   - Dark GitHub-themed styling
   - Responsive layout
   - Syntax-highlighted SQL queries
   - Visual indicators for slow queries

## Usage example

```bash
./compare-tool \
  -url1 https://api.prod.example.com/graphql \
  -url2 https://api.staging.example.com/graphql \
  -data sample-request.json \
  -output comparison.html \
  -timeout 15s
```

## Requirements

- Docker running with containers logging
- APIs must return X-Request-Id header
- Logs must include request_id field
- SQL queries logged with [sql]: prefix

## Architecture

```
docker-log-parser/
├── cmd/compare-tool/     # Comparison CLI tool
│   └── main.go
├── pkg/logs/             # Shared library
│   ├── docker.go         # Docker integration
│   └── parser.go         # Log parsing
├── main.go               # Web server (uses pkg/logs)
├── docker.go             # Main package (for tests)
├── parser.go             # Main package (for tests)
└── web/                  # Web UI
```

## Testing

- All existing tests pass
- Both tools build successfully
- Example scripts work correctly
- Sample output validates HTML generation
