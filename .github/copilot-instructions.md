# GitHub Copilot Instructions for Docker Log Viewer

This repository is a real-time Docker log monitoring and analysis tool with SQL query insights.

## Quick Reference

### Build and Test
```bash
# Build all tools
./build.sh

# Build individually
go build -o docker-log-viewer cmd/viewer/main.go
go build -o compare cmd/compare/main.go

# Run tests
go test ./...

# Run viewer
./docker-log-viewer
# Access at http://localhost:9000
```

### Environment
```bash
# Optional: PostgreSQL for EXPLAIN feature
export DATABASE_URL="postgresql://user:pass@localhost:5432/db"
```

## Architecture

### Commands (cmd/)
- **cmd/viewer**: Web-based log viewer with WebSocket streaming
- **cmd/compare**: CLI tool for comparing API endpoints

### Packages (pkg/)
- **pkg/logs**: Docker client integration and log parsing
  - `docker.go`: Docker API, container monitoring, log streaming
  - `parser.go`: Log format parsing (key=value, JSON, structured)
- **pkg/sqlexplain**: PostgreSQL EXPLAIN functionality
  - `explain.go`: Database connection, query execution, variable substitution

### Frontend (web/)
- Vue 3 single-page application
- Fully reactive data binding with Vue's composition API
- Real-time WebSocket updates
- SQL analyzer panel with PEV2-powered EXPLAIN visualization
- Template-based rendering (no manual DOM manipulation)

## Code Conventions

### Go Style
- Use context for cancellation in all async operations
- Mutex protection for shared state (especially WebSocket broadcasts)
- WebSocket message types: `log`, `containers`
- Log entries limited to 100KB (larger entries are skipped)
- Container monitoring polling interval: 5 seconds

### Vue.js Frontend
- Single Vue 3 application with reactive data
- Use computed properties for derived state
- Use v-model for two-way data binding
- Template-based rendering with v-for and v-if directives
- Methods for event handlers
- Separate PEV2 Vue app for EXPLAIN modal
- **Never** use manual DOM manipulation

### Log Parser Rules
- Custom key=value parser supporting quoted strings, nested braces, escapes
- First `key=` occurrence determines message boundary
- Dotted keys supported: `db.error`, `db.table`
- Multiple log formats: structured, JSON, timestamps, log levels

## Common Development Tasks

### Adding a new filter type
1. Add reactive state to Vue app's `data()` 
2. Update `shouldShowLog()` computed property or method
3. Add UI controls in template with v-model
4. Event handlers automatically wired via Vue directives

### Parsing a new log format
1. Update regex patterns in `pkg/logs/parser.go`
2. Add field extraction in `ParseLogLine()`
3. Update `LogEntry` struct if needed (add new fields)

### Adding a WebSocket message type
1. Define message struct in `cmd/viewer/main.go`
2. Add broadcast function for the message type
3. Handle in `ws.onmessage` in Vue app
4. Update reactive data in the handler

## Key Features

### 1. Real-time Log Streaming
- Monitors all running Docker containers simultaneously
- WebSocket-based push updates to frontend
- Auto-detects new/stopped containers (5s polling interval)
- Strips Docker stream multiplexing headers automatically

### 2. Log Parsing
Supports multiple log formats:
- **Structured**: `key=value` pairs with quoted strings, nested JSON, arrays
- **Timestamps**: Various formats like `Oct 3 19:57:52.076536`
- **Log levels**: DBG, TRC, INF, WRN, ERR, FATAL
- **File locations**: e.g., `pkg/handlers/stripe.go:85`
- **ANSI stripping**: Removes color escape sequences

### 3. Filtering
- Container selection (grouped by Docker Compose project)
- Log level filtering (DBG, TRC, INF, WRN, ERR, FATAL)
- Live text search
- Trace filtering by clicking request_id, span_id, trace_id in logs

### 4. SQL Query Analyzer
Auto-activates when filtering by trace/request/span:
- Query statistics (count, duration, slow query detection)
- N+1 detection (queries executed >5 times)
- Tables accessed in queries
- PostgreSQL EXPLAIN plans with PEV2 visualization (requires DATABASE_URL)
- Interactive execution plan tree with cost analysis

### 5. Log Details Modal
- Click any log entry to view full details
- ANSI colors converted to HTML for display
- All parsed fields shown
- JSON pretty-printing

## Testing

```bash
# Run all tests
go test ./...

# Test specific packages
go test ./pkg/logs
go test ./pkg/sqlexplain
```

### Manual Testing Checklist
- Multiple containers (use Docker Compose)
- Various log formats (structured, JSON, plain text)
- ANSI color codes in logs
- Container start/stop events

## Performance Considerations

- Backend stores last 10,000 logs in memory
- Frontend displays last 1,000 filtered logs via computed property
- WebSocket broadcast to all connected clients
- Vue's virtual DOM provides efficient rendering
- Reactive updates only re-render affected components

## Known Limitations

- Log lines >100KB are skipped (not parsed or displayed)
- Docker multiplexing assumes standard streams: stdout/stderr/stdin (0/1/2)
- Trace filtering requires exact match (no partial matching)

## When Making Changes

1. **Minimal changes**: Make the smallest possible modifications to achieve the goal
2. **Test early**: Run relevant tests after each change
3. **Build verification**: Build affected commands to catch compilation errors
4. **Manual testing**: For UI changes, test in browser with running containers
5. **Documentation**: Update docs if changing user-facing behavior
6. **Existing patterns**: Follow the patterns already established in the codebase

## Dependencies

- Go 1.21+
- Docker daemon running (for log viewer functionality)
- PostgreSQL (optional, for EXPLAIN feature only)

## Additional Documentation

For more detailed information, see:
- [AGENTS.md](../AGENTS.md) - Complete developer guide
- [cmd/analyze/README.md](../cmd/analyze/README.md) - Query analysis tool documentation
- [pkg/sqlexplain/README.md](../pkg/sqlexplain/README.md) - SQL explain plan analyzers
