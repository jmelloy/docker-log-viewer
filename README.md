# Docker Log Parser

Real-time Docker container log viewer with SQL query analysis.

## Quick Start

```bash
# Build both tools
./build.sh

# Or build individually
go build -o docker-log-viewer cmd/viewer/main.go
go build -o compare cmd/compare/main.go

# Run viewer
./docker-log-viewer
```

Open [http://localhost:9000](http://localhost:9000)

## Features

- **Real-time streaming** - Monitor all Docker containers simultaneously
- **Smart parsing** - Structured logs (key=value), JSON, timestamps, log levels
- **Interactive filtering** - Container selection, log level, live search, trace filtering
- **SQL analysis** - Query statistics, N+1 detection, slowest queries
- **EXPLAIN plans** - PostgreSQL execution plan analysis (requires DB connection)

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

### Comparison Tool

Compare API endpoints by analyzing logs and SQL performance.

```bash
go build -o compare cmd/compare/main.go
./compare -url1 <url1> -url2 <url2> -data request.json
```

Generates HTML report with SQL statistics and performance comparison.

## Requirements

- Go 1.21+
- Docker daemon running
- PostgreSQL (optional, for EXPLAIN feature)

## Documentation

- [Usage Guide](docs/USAGE_GUIDE.md) - Detailed feature walkthrough
- [SQL EXPLAIN](docs/SQL_EXPLAIN.md) - Query analysis setup
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
