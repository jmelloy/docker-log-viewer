# URL Comparison Tool

Posts GraphQL/JSON to two URLs, collects Docker logs, analyzes SQL performance.

## Quick Start

```bash
go build -o compare-tool compare.go docker.go parser.go
./compare-tool -url1 <url1> -url2 <url2> -data request.json
```

## Flags

- `-url1`, `-url2` - URLs to compare (required)
- `-data` - JSON/GraphQL file (required)
- `-output` - HTML output file (default: `comparison.html`)
- `-timeout` - Log collection timeout (default: `10s`)

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
- Response times, status codes
- SQL stats: count, duration, slow queries, N+1 detection  
- Full logs for each request
