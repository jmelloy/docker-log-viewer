# Docker Log Parser

A web-based application for monitoring and analyzing Docker container logs in real-time.

## Features

- **Real-time Streaming**: Monitor logs from all running Docker containers simultaneously
- **Smart Parsing**: Automatically detects and parses:
  - Structured logs with `key=value` format
  - JSON logs
  - Standard log levels (DBG, INF, WRN, ERR, etc.)
  - Timestamps and file locations
- **Interactive Search**: Filter logs with live search
- **Container Filtering**: Toggle individual containers on/off to focus on what matters
- **Color-coded Output**: Easy-to-read logs with syntax highlighting
- **SQL Query Analyzer**: Automatically extracts and analyzes SQL queries from logs
  - Query statistics (total, unique, duration)
  - Slowest queries identification
  - N+1 query detection
  - **EXPLAIN Plan Analysis**: Run PostgreSQL EXPLAIN on queries to understand execution plans (see [SQL_EXPLAIN.md](SQL_EXPLAIN.md))

## Installation

```bash
# Install dependencies
go mod download

# Build the application
go build -o docker-log-parser

# Run
./docker-log-parser
```

## Usage

### Running the Application

```bash
# Basic usage
./docker-log-parser

# With PostgreSQL for EXPLAIN feature
export DATABASE_URL="postgresql://user:password@localhost:5432/database"
./docker-log-parser
```

Then open your browser to `http://localhost:9000`

For detailed EXPLAIN feature setup, see [SQL_EXPLAIN.md](SQL_EXPLAIN.md).

### Features

- **Container Filtering**: Click containers to toggle their log visibility
- **Live Search**: Type in the search box to filter logs in real-time
- **Trace Filtering**: Click on any field value (request_id, span_id, trace_id) to filter logs by that value
- **SQL Query Analysis**: When filtering by trace/request/span ID, view SQL query statistics
- **EXPLAIN Plans**: Click "Run EXPLAIN" on any query to see its PostgreSQL execution plan
- **Real-time Updates**: Logs stream in real-time via WebSocket
- **Auto-scroll**: Automatically scrolls to show the latest logs

### Interface

1. **Left Sidebar**:
   - Container list with checkboxes
   - Search input
   - Active trace filter display
   - Clear logs button

2. **Main Area**: 
   - Live log display with color-coded levels
   - Click any field value to filter by it
   - Auto-scrolling to latest entries

3. **SQL Analyzer Panel** (when filtering by trace/request/span):
   - Query statistics and analysis
   - "Run EXPLAIN" buttons for execution plan analysis
   - N+1 query detection

## Log Parsing

The parser recognizes several log formats:

### Key=Value Format
```
Oct  3 19:57:52.076536 DBG pkg/handlers/stripe.go:85 > received stripe event event={...} request_id=b465d1eb
```

Parsed fields:
- Timestamp: `Oct 3 19:57:52.076536`
- Level: `DBG`
- File: `pkg/handlers/stripe.go:85`
- Message: `received stripe event`
- Fields: `event`, `request_id`

### JSON Format
```json
{"timestamp":"2025-10-03T19:57:52Z","level":"info","message":"Request processed"}
```

### Plain Text
Any text that doesn't match structured formats is displayed as-is.

## Requirements

- Go 1.21 or higher
- Docker daemon running
- Access to Docker socket (usually requires running as root or being in the `docker` group)

## Architecture

- `main.go`: TUI application and event handling
- `docker.go`: Docker client integration and log streaming
- `parser.go`: Log parsing and formatting logic

## License

MIT
