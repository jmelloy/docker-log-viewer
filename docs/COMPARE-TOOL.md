# URL Comparison Tool

Posts GraphQL/JSON to two URLs, collects Docker logs, analyzes SQL performance.

## Quick Start

```bash
go build -o compare cmd/compare/main.go
./compare -url1 <url1> -url2 <url2> -data request.json
```

## Flags

- `-url1`, `-url2` - URLs to compare (required)
- `-data` - JSON/GraphQL file (required)
- `-output` - HTML output file (default: `comparison.html`)
- `-timeout` - Log collection timeout (default: `10s`)
- `-token` - Bearer token for auth (or env: `BEARER_TOKEN`)
- `-dev-id` - X-Glue-Dev-Id header (or env: `X_GLUE_DEV_ID`)

## Requirements

- Docker running with logging containers
- APIs return `X-Request-Id` header  
- Logs include `request_id` field
- SQL logged with `[sql]:` prefix

Example log:
```
DBG db.go:12 > [sql]: SELECT * FROM users WHERE id = $1 db.table=users duration=1.234 request_id=abc123
```

## Output

HTML report with:
- **Request payload** - Collapsible JSON/GraphQL viewer
- Response times, status codes
- SQL stats: count, duration, slow queries, N+1 detection  
- Full logs for each request
