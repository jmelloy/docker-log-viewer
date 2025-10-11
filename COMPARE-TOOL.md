# URL Comparison Tool

A tool for comparing GraphQL/JSON API endpoints by analyzing their logs and SQL performance.

## Quick Start

```bash
# 1. Build the tool
go build -o compare-tool ./cmd/compare-tool

# 2. Make sure Docker containers are running
docker ps

# 3. Create a request payload file (JSON or GraphQL)
cat > my-request.json <<EOF
{
  "query": "query GetUser(\$id: ID!) { user(id: \$id) { name email } }",
  "variables": { "id": "123" }
}
EOF

# 4. Run the comparison
./compare-tool \
  -url1 https://api-production.example.com/graphql \
  -url2 https://api-staging.example.com/graphql \
  -data my-request.json \
  -output comparison.html

# 5. Open the report in your browser
open comparison.html
```

## Features

- Posts the same GraphQL/JSON payload to two different URLs
- Captures X-Request-Id headers from responses
- Collects Docker logs for each request
- Analyzes SQL queries from logs
- Generates side-by-side HTML comparison report

## Building

```bash
# Build the compare tool
go build -o compare-tool ./cmd/compare-tool
```

## Usage

```bash
./compare-tool -url1 <first-url> -url2 <second-url> -data <json-or-graphql-file> [-output comparison.html] [-timeout 10s]
```

### Arguments

- `-url1`: First URL to test (required)
- `-url2`: Second URL to test (required)
- `-data`: Path to GraphQL or JSON data file to POST (required)
- `-output`: Output HTML file path (default: comparison.html)
- `-timeout`: Timeout for log collection after each request (default: 10s)

### Example

```bash
# Compare production vs staging endpoint
./compare-tool \
  -url1 https://api.production.com/graphql \
  -url2 https://api.staging.com/graphql \
  -data sample-request.json \
  -output prod-vs-staging.html \
  -timeout 15s
```

## Requirements

- Docker must be running
- Application containers must be running and logging
- API responses must include X-Request-Id header
- Logs must include request_id field matching the X-Request-Id header
- SQL queries should be logged with `[sql]:` prefix

## Log Format

The tool expects logs in the format used by the docker-log-viewer:

```
Oct  3 21:53:27.208471 INF pkg/observability/logging.go:110 > Processing request request_id=508e6ccc
Oct  3 21:53:27.210123 DBG pkg/database/query.go:45 > [sql]: SELECT * FROM users WHERE id = $1 db.table=users db.operation=select db.rows=1 duration=1.234 request_id=508e6ccc
```

Key fields:
- `request_id`: Must match X-Request-Id header
- `duration`: Query duration in milliseconds
- `db.table`: Database table name
- `db.operation`: SQL operation (select, insert, update, delete)
- `db.rows`: Number of rows affected

## HTML Report

The generated HTML report includes:

### For each URL:
- HTTP status code and response duration
- Number of logs collected
- Request ID

### SQL Analysis (if queries found):
- Total queries executed
- Unique query patterns
- Average and total duration
- Slowest queries (top 5)
- Most frequent queries
- N+1 query detection (queries executed >5 times)
- Tables accessed with counts

### Raw Logs:
- All collected logs for each request
- Color-coded by log level (error, warn, info)

## Troubleshooting

### No logs collected
- Ensure Docker containers are running
- Check that application is logging with request_id field
- Increase `-timeout` value

### X-Request-Id not found
- Verify API returns X-Request-Id header in response
- Check response headers manually with curl

### No SQL queries detected
- Ensure queries are logged with `[sql]:` prefix
- Check log format matches expected pattern
- Verify duration and db.* fields are present

## Integration with docker-log-viewer

This tool uses the same log parsing logic as the main docker-log-viewer application. It can run alongside the viewer without conflicts.
