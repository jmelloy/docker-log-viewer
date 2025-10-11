# Docker Log Parser - Agent Guide

## Overview

A web-based Docker log monitoring and analysis tool with real-time streaming, advanced filtering, and SQL query analysis capabilities.

## Quick Start

```bash
# Build
go build -o docker-log-parser

# Run
./docker-log-parser

# Access
http://localhost:9000
```

## Architecture

### Backend (Go)
- **main.go**: Web server, WebSocket handling, container monitoring
- **docker.go**: Docker client integration, log streaming with multiplexing support
- **parser.go**: Log parsing (key=value format, JSON, structured logs)

### Frontend (Web)
- **web/index.html**: UI structure
- **web/style.css**: Dark GitHub-themed styling
- **web/app.js**: Application logic, real-time updates, SQL analysis

## Key Features

### 1. Real-time Log Streaming
- Monitors all running Docker containers
- WebSocket-based live updates
- Automatically detects new/stopped containers (5s polling)
- Strips Docker stream multiplexing headers (8-byte frames)

### 2. Log Parsing
- **Structured logs**: `key=value` format with support for:
  - Quoted strings: `error="record not found"`
  - Nested JSON: `event={...}`
  - Arrays: `location=[...]`
  - Dotted keys: `db.error`, `db.table`
- **Timestamps**: `Oct 3 19:57:52.076536`
- **Log levels**: DBG, TRC, INF, WRN, ERR, FATAL (and long forms)
- **File locations**: `pkg/handlers/stripe.go:85`
- **ANSI color stripping**: Removes all escape sequences

### 3. Filtering & Search
- **Container filtering**: Group by Docker Compose project, collapsible tree view
- **Log level filtering**: Toggle individual levels (TRC < DBG < INF < WRN < ERR)
- **Live search**: Real-time text search across all fields
- **Trace filtering**: Click any field value (request_id, span_id, trace_id) to filter

### 4. SQL Query Analyzer
- Automatically activates when filtering by trace/request/span ID
- Extracts SQL from `[sql]:` log entries
- **Overview stats**: Total queries, unique queries, avg/total duration
- **Slowest queries**: Top 5 by duration with table/operation/rows
- **Most frequent**: Identifies repeated queries
- **N+1 detection**: Flags queries executed >5 times
- **Tables accessed**: Shows all tables with query counts

### 5. Log Details Modal
- Click any log line to view full details
- **Raw log**: ANSI colors converted to HTML
- **Parsed fields**: All extracted key=value pairs
- **JSON pretty-print**: Auto-formats nested JSON objects
- Close with: × button, click outside, or Escape key

## File Structure

```
docker-log-parser/
├── main.go                 # Web server & WebSocket
├── cmd/
│   └── compare-tool/       # URL comparison CLI tool
│       └── main.go
├── pkg/
│   └── logs/               # Shared log parsing library
│       ├── docker.go       # Docker integration
│       └── parser.go       # Log parsing logic
├── go.mod                  # Go dependencies
├── web/
│   ├── index.html          # UI layout
│   ├── style.css           # Styling
│   └── app.js              # Frontend logic
├── AGENTS.md               # This file
└── COMPARE-TOOL.md         # Comparison tool docs
```

## Code Conventions

### Go
- Use context for cancellation
- Mutex protection for shared state (logs, clients, containers)
- WebSocket message types: `log`, `containers`
- Log entries limited to 100KB to prevent corruption
- Container monitoring every 5 seconds

### JavaScript
- ES6 class-based architecture
- Set for efficient container/level selection
- Regex normalization for SQL query grouping
- ANSI escape code regex: `\x1b\[([0-9;]+)m`

### Parser Rules
- Custom key=value parser handles:
  - Quoted strings with escapes
  - Nested braces/brackets with depth tracking
  - Unquoted values stop at spaces
- First occurrence of `key=` determines message boundary
- Field values can contain spaces if quoted/bracketed

## Common Patterns

### Adding a new filter type
1. Add state to `App` class (e.g., `this.myFilter`)
2. Update `shouldShowLog()` method
3. Add UI controls in `index.html`
4. Wire up event listeners in `setupEventListeners()`

### Parsing a new log format
1. Update regex patterns in `parser.go`
2. Add field extraction logic in `ParseLogLine()`
3. Update `LogEntry` struct if needed

### Adding WebSocket message type
1. Define struct in `main.go` (e.g., `type MyMessage struct`)
2. Add broadcast function (e.g., `broadcastMyUpdate()`)
3. Handle in frontend `ws.onmessage` handler
4. Add handler method in `app.js`

## Testing

No automated tests yet. Manual testing with:
- Multiple containers (Docker Compose)
- Log files with various formats (JSON, structured, plain text)
- ANSI color codes
- Container start/stop events

## Known Issues

- Very long log lines (>100KB) are skipped
- Docker multiplexing assumes stdout/stderr/stdin (0/1/2)
- Trace filter requires exact match (no partial matching)

## Dependencies

### Backend
- `github.com/docker/docker`: Docker API client
- `github.com/gorilla/websocket`: WebSocket support

### Frontend
- Vanilla JavaScript (no frameworks)
- CSS Grid/Flexbox for layout

## Performance

- Stores last 10,000 logs in memory (trimmed to 1,000 when exceeded)
- Displays last 1,000 filtered logs in UI
- WebSocket broadcast to all connected clients
- Container list limited to 300px height with scroll

## Future Enhancements

Consider adding:
- Log export (JSON, CSV)
- Persistent trace filter history
- Regex search support
- Log aggregation/grouping by time windows
- Metrics dashboard
- Alert rules
