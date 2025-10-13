# Docker Log Parser

Real-time Docker container log viewer with SQL query analysis and GraphQL request management.

## Quick Start

```bash
# Build all tools
./build.sh

# Or build individually
go build -o docker-log-viewer cmd/viewer/main.go
go build -o compare cmd/compare/main.go
go build -o graphql-tester cmd/graphql-tester/main.go
go build -o analyze cmd/analyze/main.go

# Run viewer
./docker-log-viewer
```

Open [http://localhost:9000](http://localhost:9000)

## Features

- **Real-time streaming** - Monitor all Docker containers simultaneously
- **Smart parsing** - Structured logs (key=value), JSON, timestamps, log levels
- **Interactive filtering** - Container selection, log level, live search, trace filtering
- **SQL analysis** - Query statistics, N+1 detection, slowest queries
- **EXPLAIN plans** - PostgreSQL execution plan visualization with PEV2 (requires DB connection)
- **Request Management** - Save, execute, and analyze GraphQL/API requests
- **Before/After Analysis** - Track request performance over time
- **Vue.js UI** - Fully reactive interface with automatic updates

## Tools

### Log Viewer

Web-based real-time log viewer with filtering and SQL analysis.

```bash
go build -o docker-log-viewer cmd/viewer/main.go
./docker-log-viewer

# With PostgreSQL for EXPLAIN feature
export DATABASE_URL="postgresql://user:pass@localhost:5432/db"
./docker-log-viewer
```

### GraphQL Request Manager

Save and execute GraphQL/API requests with full log capture and SQL analysis.

**Web UI**: [http://localhost:9000/requests.html](http://localhost:9000/requests.html)

**CLI**:
```bash
# Save a request
./graphql-tester -url "https://api.example.com/graphql" \
                 -data graphql-operations-unique/AuthConfig.json

# Execute immediately
./graphql-tester -url "https://api.example.com/graphql" \
                 -data operations/query.json \
                 -execute

# List saved requests
./graphql-tester -list
```

See [docs/GRAPHQL_MANAGER.md](docs/GRAPHQL_MANAGER.md) for full documentation.

### Comparison Tool

Compare API endpoints by analyzing logs and SQL performance.

```bash
go build -o compare cmd/compare/main.go
./compare -url1 <url1> -url2 <url2> -data request.json
```

Generates HTML report with SQL statistics and performance comparison.

### Query Analysis Tool

Compare SQL queries from two saved execution IDs.

```bash
go build -o analyze cmd/analyze/main.go
./analyze -exec1 1 -exec2 2

# Save to file
./analyze -exec1 1 -exec2 2 -output report.txt

# Verbose mode with all queries
./analyze -exec1 1 -exec2 2 -verbose
```

Analyzes query performance, identifies regressions, and provides index recommendations.
See [cmd/analyze/README.md](cmd/analyze/README.md) for full documentation.

## Requirements

- Go 1.21+
- Docker daemon running
- PostgreSQL (optional, for EXPLAIN feature)

## Documentation

- [Usage Guide](docs/USAGE_GUIDE.md) - Detailed feature walkthrough
- [SQL EXPLAIN](docs/SQL_EXPLAIN.md) - Query analysis setup
- [PEV2 Integration](docs/PEV2_INTEGRATION.md) - PostgreSQL EXPLAIN visualization
- [GraphQL Manager](docs/GRAPHQL_MANAGER.md) - Request management guide
- [Comparison Tool](docs/COMPARE-TOOL.md) - API comparison guide
- [Implementation](docs/IMPLEMENTATION.md) - Technical details
- [AGENTS.md](AGENTS.md) - Developer guide

## Architecture

```
docker-log-parser/
├── cmd/
│   ├── viewer/         # Web-based log viewer
│   └── compare/        # URL comparison tool
├── pkg/
│   ├── logs/           # Docker & log parsing
│   └── sqlexplain/     # PostgreSQL EXPLAIN
└── web/                # Frontend assets
```

## License

MIT
