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

## ANSI-Aware Field Boundary Parsing

**File**: `ansi_field_parsing_demo.go`

Demonstrates how the log parser uses ANSI escape codes as hints for identifying field boundaries in key=value structured logs.

### Running the Example

```bash
go run examples/ansi_field_parsing_demo.go
```

### What It Shows

The example demonstrates how ANSI codes help with field parsing:
- **ANSI around field names**: Colors highlighting field names serve as boundary markers
- **ANSI in field values**: Colored values are parsed correctly with ANSI codes stripped
- **ANSI as delimiters**: Reset codes between fields help identify boundaries
- **Fallback**: Regular parsing when no ANSI codes are present

This is particularly useful for logs from applications that use colored output, where spacing alone might be ambiguous.

### Benefits
- Better parsing of colored log output
- ANSI codes serve as additional field boundary hints
- Automatic fallback to regular parsing
- Works seamlessly with various log formats

## Key Functions Used

### Log Entry Boundary Detection

- `logs.IsLikelyNewLogEntry(line)`: Determines if a line is a new log entry using multiple heuristics:
  - Checks for ANSI codes at the start
  - Checks for timestamps near the beginning
  - Checks for log levels at the start
  - Rejects lines with leading whitespace (continuation lines)

- `logs.startsWithANSI(line)`: Checks if a line starts with an ANSI escape code
- `logs.hasANSICodes(line)`: Checks if a line contains any ANSI codes

### Field Boundary Parsing

- `parseKeyValuePairsWithANSI(s)`: Parses key=value pairs using ANSI codes as boundary hints
- `parseKeyValuePairs(s)`: Standard key=value parser (fallback)

### Example Output

#### Log Entry Delineation
```
>>> Line  1: IsNewEntry= true | "\x1b[32mOct  3 21:53:27.208471\x1b[0m \x1b[34mINF\x1b[0m Application..."
    Line  2: IsNewEntry=false | "  Loading configuration from /etc/app/config.yaml"
    Line  3: IsNewEntry=false | "  Connecting to database at localhost:5432"
>>> Line  4: IsNewEntry= true | "\x1b[32mOct  3 21:53:28.456789\x1b[0m \x1b[33mWARN\x1b[0m..."
```

`>>>` indicates a new log entry, while indented lines without the marker are continuation lines.

#### Field Boundary Parsing
```
Raw: "\x1b[33mrequest_id\x1b[0m=abc123 \x1b[36muser_id\x1b[0m=456"
Fields:
  request_id = abc123
  user_id = 456
```

ANSI codes around field names help identify where each field begins, even without clear spacing.

