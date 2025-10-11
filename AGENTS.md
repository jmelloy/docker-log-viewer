# Docker Log Parser - Agent Guide

## Overview

Real-time Docker log monitoring and analysis tool with SQL query insights.

## Quick Start

```bash
# Build viewer
go build -o docker-log-viewer cmd/viewer/main.go

# Run
./docker-log-viewer

# Access
http://localhost:9000
```

## Build Commands

```bash
# Log viewer
go build -o docker-log-viewer cmd/viewer/main.go

# Comparison tool
go build -o compare cmd/compare/main.go

# Run tests
go test ./...
```

## Architecture

### Commands
- **cmd/viewer**: Web-based log viewer with WebSocket streaming
- **cmd/compare**: CLI tool for comparing API endpoints

### Packages
- **pkg/logs**: Docker client integration and log parsing
  - `docker.go`: Docker API, container monitoring, log streaming
  - `parser.go`: Log format parsing (key=value, JSON, structured)
- **pkg/sqlexplain**: PostgreSQL EXPLAIN functionality
  - `explain.go`: Database connection, query execution, variable substitution

### Frontend
- **web/**: HTML, CSS, JavaScript for UI
  - Real-time WebSocket updates
  - SQL analyzer panel
  - EXPLAIN modal

## Key Features

### 1. Real-time Log Streaming
- Monitors all running Docker containers
- WebSocket-based updates
- Auto-detects new/stopped containers (5s polling)
- Strips Docker stream multiplexing headers

### 2. Log Parsing
Supports multiple formats:
- **Structured**: `key=value` with quoted strings, nested JSON, arrays
- **Timestamps**: `Oct 3 19:57:52.076536`
- **Log levels**: DBG, TRC, INF, WRN, ERR, FATAL
- **File locations**: `pkg/handlers/stripe.go:85`
- **ANSI stripping**: Removes color escape sequences

### 3. Filtering
- Container selection (grouped by Docker Compose project)
- Log level filtering
- Live text search
- Trace filtering (click request_id, span_id, trace_id)

### 4. SQL Query Analyzer
Auto-activates when filtering by trace/request/span:
- Query stats (count, duration, slow queries)
- N+1 detection (queries executed >5 times)
- Tables accessed
- PostgreSQL EXPLAIN plans (requires DATABASE_URL)

### 5. Log Details Modal
- Click any log to view full details
- ANSI colors converted to HTML
- Parsed fields
- JSON pretty-print

## Code Conventions

### Go
- Use context for cancellation
- Mutex protection for shared state
- WebSocket message types: `log`, `containers`
- Log entries limited to 100KB
- Container monitoring every 5 seconds

### JavaScript
- ES6 class-based architecture
- Set for efficient selection
- Regex normalization for SQL grouping
- ANSI escape: `\x1b\[([0-9;]+)m`

### Parser Rules
- Custom key=value parser with quoted strings, nested braces, escapes
- First `key=` occurrence determines message boundary
- Dotted keys supported: `db.error`, `db.table`

## Common Tasks

### Add a filter type
1. Add state to `App` class
2. Update `shouldShowLog()` method
3. Add UI controls in `index.html`
4. Wire event listeners in `setupEventListeners()`

### Parse a new format
1. Update regex in `pkg/logs/parser.go`
2. Add extraction in `ParseLogLine()`
3. Update `LogEntry` struct if needed

### Add WebSocket message
1. Define struct in `cmd/viewer/main.go`
2. Add broadcast function
3. Handle in `ws.onmessage`
4. Add handler in `app.js`

## Testing

```bash
# Run all tests
go test ./...

# Specific package
go test ./pkg/logs
go test ./pkg/sqlexplain
```

Manual testing:
- Multiple containers (Docker Compose)
- Various log formats
- ANSI color codes
- Container start/stop events

## Performance

- Last 10,000 logs in memory
- UI displays last 1,000 filtered logs
- WebSocket broadcast to all clients
- Container list scroll at 300px

## Environment

```bash
# Optional: PostgreSQL for EXPLAIN feature
export DATABASE_URL="postgresql://user:pass@localhost:5432/db"
```

## Known Limitations

- Log lines >100KB are skipped
- Docker multiplexing assumes stdout/stderr/stdin (0/1/2)
- Trace filter requires exact match
