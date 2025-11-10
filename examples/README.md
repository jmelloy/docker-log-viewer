# Examples

This directory contains example programs demonstrating features of the Docker Log Parser.

## ANSI Escape Code Delineation

**File**: `ansi_delineation_demo.go`

Demonstrates how the log parser uses ANSI escape codes as delineators to identify log entry boundaries in multi-line log streams.

### Running the Example

```bash
go run examples/ansi_delineation_demo.go
```

### What It Shows

The example demonstrates how the parser distinguishes between:
- **New log entries**: Lines starting with ANSI codes (`\x1b[32m`), timestamps, or log levels
- **Continuation lines**: Lines with leading whitespace, no ANSI codes at start

This feature enables correct grouping of multi-line logs such as:
- SQL queries spanning multiple lines
- Stack traces
- Multi-line error messages
- Configuration dumps

### Key Functions Used

- `logs.IsLikelyNewLogEntry(line)`: Determines if a line is a new log entry using multiple heuristics:
  - Checks for ANSI codes at the start
  - Checks for timestamps near the beginning
  - Checks for log levels at the start
  - Rejects lines with leading whitespace (continuation lines)

- `logs.startsWithANSI(line)`: Checks if a line starts with an ANSI escape code
- `logs.hasANSICodes(line)`: Checks if a line contains any ANSI codes

### Example Output

```
>>> Line  1: IsNewEntry= true | "\x1b[32mOct  3 21:53:27.208471\x1b[0m \x1b[34mINF\x1b[0m Application..."
    Line  2: IsNewEntry=false | "  Loading configuration from /etc/app/config.yaml"
    Line  3: IsNewEntry=false | "  Connecting to database at localhost:5432"
>>> Line  4: IsNewEntry= true | "\x1b[32mOct  3 21:53:28.456789\x1b[0m \x1b[33mWARN\x1b[0m..."
```

`>>>` indicates a new log entry, while indented lines without the marker are continuation lines.
