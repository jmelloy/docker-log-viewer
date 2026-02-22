# Docker Log Viewer - AI Assistant Guide

> **Purpose**: This document provides AI assistants with comprehensive context about the Docker Log Viewer codebase structure, development workflows, and conventions to follow when making changes.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Quick Start](#quick-start)
3. [Codebase Structure](#codebase-structure)
4. [Technology Stack](#technology-stack)
5. [Architecture & Design Patterns](#architecture--design-patterns)
6. [Development Workflows](#development-workflows)
7. [Code Conventions](#code-conventions)
8. [Testing Strategy](#testing-strategy)
9. [Common Tasks](#common-tasks)
10. [File Location Reference](#file-location-reference)
11. [Key Constraints & Limitations](#key-constraints--limitations)

---

## Project Overview

**Docker Log Viewer** is a real-time Docker container log monitoring and analysis tool with SQL query insights. It provides:

- **Real-time log streaming** from all Docker containers via WebSocket
- **Advanced log parsing** supporting multiple formats (key=value, JSON, timestamps, log levels)
- **Smart filtering** by container, log level, text search, and trace IDs
- **SQL query analysis** with statistics, N+1 detection, and PostgreSQL EXPLAIN plan visualization
- **GraphQL/API request management** for tracking and comparing request performance
- **Performance comparison** tools for analyzing query differences between executions

**Primary Use Cases**:
- Development environment log monitoring
- Performance debugging and SQL query optimization
- Request/response analysis for GraphQL/REST APIs
- Before/after comparison for code changes

---

## Quick Start

### Building the Project

```bash
# Build everything (frontend + all Go binaries)
./build.sh

# Or build individually
go build -o docker-log-viewer cmd/viewer/main.go
go build -o graphql-tester cmd/graphql-tester/main.go
go build -o analyze cmd/analyze/main.go

# Frontend only
cd web && npm install && npm run build
```

### Running the Application

```bash
# Development with Docker Compose (recommended)
docker-compose up

# Or run locally
./docker-log-viewer

# Access web UI at http://localhost:9000
```

### Development Commands

```bash
# Format all code
make format

# Check formatting
make format-check

# Lint JavaScript
make lint
make lint-fix

# Run tests
make test
go test ./...
```

---

## Codebase Structure

```
docker-log-viewer/
├── cmd/                              # Executable commands
│   ├── viewer/                       # Main web server (WebSocket + HTTP API)
│   │   └── main.go                   # 2,991 lines - refactored, organized
│   ├── graphql-tester/               # CLI for saving/executing requests
│   │   └── main.go                   # 418 lines - uses httputil/sqlutil
│   ├── analyze/                      # Query comparison tool
│   │   └── main.go                   # Compares SQL between executions
│   └── test-parser/                  # Parser testing utility
│
├── pkg/                              # Core libraries
│   ├── controller/                   # HTTP request handlers (NEW pattern)
│   │   ├── controller.go             # Main controller with all handlers
│   │   ├── logs.go                   # Log retrieval endpoints
│   │   ├── containers.go             # Container management
│   │   ├── requests.go               # Request CRUD operations
│   │   ├── servers.go                # Server configuration
│   │   ├── sql.go                    # SQL query stats & EXPLAIN
│   │   └── samples.go                # Sample query templates
│   │
│   ├── logs/                         # Docker integration
│   │   ├── docker.go                 # Docker client, container monitoring
│   │   ├── parser.go                 # Advanced log format parsing
│   │   └── timestamp.go              # Timestamp parsing utilities
│   │
│   ├── logstore/                     # Indexed log storage
│   │   └── logstore.go               # In-memory log storage (10k max)
│   │
│   ├── sqlexplain/                   # PostgreSQL analysis
│   │   ├── explain.go                # DB connection, query execution
│   │   ├── analyzer.go               # Query plan comparison
│   │   └── index_analyzer.go         # Index usage analysis
│   │
│   ├── store/                        # SQLite persistence
│   │   ├── store.go                  # GORM models
│   │   └── migrations/               # 23 sequential migrations
│   │
│   ├── httputil/                     # HTTP utilities (NEW)
│   │   ├── httputil.go               # Request ID, HTTP POST, log collection
│   │   └── httputil_test.go          # Unit tests
│   │
│   ├── sqlutil/                      # SQL utilities (NEW)
│   │   ├── sqlutil.go                # Query extraction, interpolation, formatting
│   │   └── sqlutil_test.go           # Unit tests
│   │
│   └── utils/                        # General utilities
│       └── sql.go                    # SQL parsing helpers
│
├── web/                              # Vue 3 frontend
│   ├── src/
│   │   ├── views/                    # Page-level components
│   │   │   ├── LogsView.vue          # Real-time log streaming
│   │   │   ├── RequestsView.vue      # Request management
│   │   │   ├── RequestDetailView.vue # Execution details + SQL analysis
│   │   │   ├── SqlDetailView.vue     # SQL query analysis
│   │   │   ├── GraphQLExplorerView.vue # GraphQL editor
│   │   │   └── SettingsView.vue      # Configuration
│   │   │
│   │   ├── components/               # Reusable UI components
│   │   │   ├── AppHeader.vue         # Top navigation
│   │   │   ├── AppNav.vue            # Sidebar
│   │   │   ├── LogStream.vue         # Log display
│   │   │   ├── ExplainPlanFormatter.vue # EXPLAIN visualization
│   │   │   ├── PlanNodeItem.vue      # Plan node rendering
│   │   │   └── SimpleQueryPlanViewer.vue # Simplified plan tree
│   │   │
│   │   ├── router/                   # Vue Router config
│   │   │   └── index.ts              # Route definitions
│   │   │
│   │   ├── utils/                    # Utilities
│   │   │   ├── api.ts                # Fetch-based API client
│   │   │   ├── graphql-editor-manager.ts # CodeMirror setup
│   │   │   ├── codemirror-graphql.ts # CodeMirror bundle
│   │   │   └── ui-utils.ts           # Formatting helpers
│   │   │
│   │   └── types/                    # TypeScript definitions
│   │       └── index.ts              # Shared types
│   │
│   ├── static/                       # Static assets, libraries
│   ├── dist/                         # Production build output
│   ├── package.json                  # Frontend dependencies
│   ├── vite.config.ts                # Vite configuration
│   └── tsconfig.json                 # TypeScript config
│
├── .github/
│   └── copilot-instructions.md       # GitHub Copilot guide
├── .vscode/
│   └── settings.json                 # VS Code configuration
├── docker-compose.yml                # Development orchestration
├── Dockerfile                        # Production build
├── Dockerfile.dev                    # Backend dev with hot reload
├── Dockerfile.dev.frontend           # Frontend dev with Vite
├── .air.toml                         # Air hot reload config
├── Makefile                          # Development commands
├── build.sh                          # Build script
├── mise.toml                         # Version management
├── .prettierrc                       # Code formatting rules
├── AGENTS.md                         # Developer guide
├── REFACTORING_GUIDE.md              # Recent refactoring notes
└── README.md                         # User documentation
```

### Recent Architectural Changes

**Major Refactoring (see REFACTORING_GUIDE.md)**:
- Created `pkg/httputil` and `pkg/sqlutil` to eliminate code duplication
- Deleted `cmd/compare` (1,066 lines removed)
- Reduced `cmd/viewer/main.go` by 407 lines (12% reduction)
- Reduced `cmd/graphql-tester/main.go` by 96 lines (19% reduction)
- **Net reduction: 1,039 lines** while improving maintainability

**Handler Organization**:
`cmd/viewer/main.go` handlers are organized into logical sections:
1. Container & Log Management
2. SQL Analysis & Trace Management
3. Request & Execution Management
4. Server & Database Configuration
5. Container Retention Settings

---

## Technology Stack

### Backend

| Technology | Version | Purpose |
|------------|---------|---------|
| **Go** | 1.26.0 | Primary backend language |
| Docker SDK | v24.0.7 | Docker API client |
| Gorilla Mux | v1.8.1 | HTTP routing |
| Gorilla WebSocket | v1.5.3 | Real-time communication |
| GORM | v1.31.0 | ORM for SQLite |
| SQLite | (via GORM) | Local database |
| PostgreSQL Driver | lib/pq v1.10.9 | EXPLAIN plan analysis |
| Goose | v3.26.0 | Database migrations |
| Tint | v1.1.2 | Structured logging |

### Frontend

| Technology | Version | Purpose |
|------------|---------|---------|
| **Vue 3** | 3.5.24 | Reactive UI framework |
| **TypeScript** | 5.9.3 | Type-safe JavaScript |
| **Vite** | 7.2.2 | Build tool & dev server |
| Vue Router | 4.6.3 | Client-side routing |
| CodeMirror 6 | 6.23.0 | GraphQL editor |
| cm6-graphql | 0.2.1 | GraphQL syntax support |
| sql-formatter | 15.6.10 | SQL formatting |
| highlight.js | 11.11.1 | Syntax highlighting |
| JSZip | 3.10.1 | File compression/export |
| vue-tippy | 6.7.1 | Tooltips |

### Development Tools

| Tool | Purpose |
|------|---------|
| **Air** | Go hot reload (.air.toml) |
| **Prettier** | Code formatting (120 char, double quotes) |
| **ESLint** | JavaScript linting |
| **vue-tsc** | TypeScript type checking |
| **mise** | Version management (Go, Python) |
| **Docker Compose** | Development orchestration |

---

## Architecture & Design Patterns

### 1. Real-Time WebSocket Architecture

**Data Flow**:
```
Docker Containers
    ↓ (Docker API)
DockerClient (pkg/logs/docker.go)
    ↓ (Log Stream)
LogParser (pkg/logs/parser.go) → Structured LogEntry
    ↓ (Indexed Storage)
LogStore (pkg/logstore) → 10,000 logs max
    ↓ (WebSocket Broadcast)
Connected Clients (Vue Frontend)
    ↓ (User Filter Application)
LogsView Component → Reactive Display
```

**Key Components**:
- **Client Connection Management**: WebSocket clients tracked in memory
- **Message Batching**: 1-second batching delay for efficiency
- **Filter Criteria**: Clients send filter preferences via WebSocket
- **Broadcast Filtering**: Server filters logs before sending to each client

### 2. Multi-Level Indexing (LogStore)

**Storage Strategy**:
- Doubly-linked list for efficient insertion/eviction
- Multi-level indexing:
  - By container ID
  - By field name/value pairs
  - By timestamp
- Configurable retention (size & age-based)
- Per-container retention policies

### 3. Vue 3 Composition API Pattern

**Component Structure**:
```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

// Reactive state
const logs = ref<LogEntry[]>([])
const filters = ref({ level: '', search: '' })

// Computed properties
const filteredLogs = computed(() => {
  return logs.value.filter(log => matchesFilters(log))
})

// Lifecycle hooks
onMounted(() => {
  initWebSocket()
})
</script>

<template>
  <div v-for="log in filteredLogs" :key="log.id">
    {{ log.message }}
  </div>
</template>
```

**Conventions**:
- Use `ref()` for primitive reactive state
- Use `computed()` for derived state
- Use `v-model` for two-way binding
- Template-based rendering only (no manual DOM manipulation)

### 4. Database-Driven Request Tracking

**Entity Relationships**:
```
Server ─┐
        ├──> Request ──> ExecutedRequest ─┬──> ExecutionLog
        │                                  └──> ExecutionSQLQuery
        │
Database ──> (EXPLAIN plans)
```

**GORM Models** (pkg/store/store.go):
- `Server`: API endpoint configurations with auth
- `Database`: PostgreSQL connection configs
- `Request`: Saved GraphQL/API request templates
- `SampleQuery`: Pre-defined query examples
- `ExecutedRequest`: Historical executions
- `ExecutionLog`: Captured logs during execution
- `SQLQuery`: Parsed SQL queries
- `ExecutionSQLQuery`: Links queries to executions

### 5. Modular Package Design

**Separation of Concerns**:
- `pkg/logs`: Docker integration & parsing (no HTTP)
- `pkg/logstore`: Storage logic (no Docker)
- `pkg/controller`: HTTP handlers (no business logic)
- `pkg/httputil`: Reusable HTTP utilities
- `pkg/sqlutil`: Reusable SQL utilities
- `pkg/sqlexplain`: PostgreSQL analysis (isolated)

**Benefits**:
- Testable in isolation
- No circular dependencies
- Clear responsibility boundaries
- Easy to mock for testing

---

## Development Workflows

### Adding a New Feature

1. **Read existing code first**
   ```bash
   # Understand the current implementation
   # Read relevant files before making changes
   ```

2. **Plan with tests in mind**
   ```bash
   # Identify what needs testing
   # Write tests alongside code when possible
   ```

3. **Follow existing patterns**
   - Use similar structure to existing handlers/components
   - Reuse utilities from `pkg/httputil` and `pkg/sqlutil`
   - Follow Vue Composition API patterns

4. **Format and lint**
   ```bash
   make format
   make lint-fix
   ```

5. **Test thoroughly**
   ```bash
   make test                    # All Go tests
   go test ./pkg/logs           # Specific package
   cd web && npm run type-check # TypeScript checks
   ```

6. **Manual testing**
   ```bash
   docker-compose up            # Run with hot reload
   # Test in browser at http://localhost:9000
   ```

### Code Formatting Standards

**Go**:
- Use `gofmt` (enforced by `make format`)
- Standard Go conventions
- Descriptive variable names
- Comments for exported functions

**TypeScript/Vue**:
- Prettier with 120 character line width
- Double quotes for strings
- Consistent indentation (2 spaces)
- Type annotations for all function parameters

**Configuration Files**:
```json
// .prettierrc
{
  "printWidth": 120,
  "tabWidth": 2,
  "semi": false,
  "singleQuote": false,
  "trailingComma": "es5"
}
```

### Git Workflow

**Branch Naming**: No specific convention enforced

**Commit Messages**:
- Concise, descriptive commits
- Focus on "why" over "what"
- Example: "Add SQL export functionality and integrate JSZip"

**Before Committing**:
```bash
make format-check   # Ensure code is formatted
make test           # Ensure tests pass
./build.sh          # Ensure everything builds
```

### Docker Compose Development

**Services**:
- `app`: Backend with Air hot reload (port 9000)
- `web`: Frontend with Vite dev server (port 5173)

**Environment Variables**:
```bash
DATABASE_URL=postgresql://user:pass@host:5432/db
NOTION_API_KEY=secret_xxx
NOTION_DATABASE_ID=xxx
```

**Common Operations**:
```bash
docker-compose up                 # Start all services
docker-compose up -d --build      # Rebuild and start detached
docker-compose logs -f app        # Follow backend logs
docker-compose logs -f web        # Follow frontend logs
docker-compose down               # Stop all services
docker-compose exec app sh        # Shell into backend container
```

---

## Code Conventions

### Go Conventions

#### Context Usage
```go
// Always use context for cancellation
func (d *DockerClient) MonitorContainers(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            d.updateContainers()
        }
    }
}
```

#### Mutex Protection
```go
// Protect shared state with mutexes
type ClientManager struct {
    mu      sync.RWMutex
    clients map[string]*Client
}

func (cm *ClientManager) AddClient(id string, client *Client) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    cm.clients[id] = client
}
```

#### Error Handling
```go
// Return errors, don't panic
func ParseLogLine(line string) (*LogEntry, error) {
    if len(line) > MaxLogSize {
        return nil, fmt.Errorf("log line exceeds max size: %d", len(line))
    }
    // ... parsing logic
    return entry, nil
}
```

#### WebSocket Message Types
```go
// Use typed messages
type LogMessage struct {
    Type      string     `json:"type"`      // "log"
    Container string     `json:"container"`
    Log       *LogEntry  `json:"log"`
}

type ContainerMessage struct {
    Type       string      `json:"type"`      // "containers"
    Containers []Container `json:"containers"`
}
```

### Vue 3 Conventions

#### Reactive State Management
```typescript
// Use ref for primitives, reactive for objects
import { ref, reactive, computed } from 'vue'

const count = ref(0)
const state = reactive({
  logs: [],
  filters: { level: '', search: '' }
})

const filteredLogs = computed(() => {
  return state.logs.filter(log => matchesFilters(log))
})
```

#### Component Props & Emits
```vue
<script setup lang="ts">
interface Props {
  log: LogEntry
  showTimestamp?: boolean
}

interface Emits {
  (e: 'click', log: LogEntry): void
  (e: 'filter', field: string, value: string): void
}

const props = withDefaults(defineProps<Props>(), {
  showTimestamp: true
})

const emit = defineEmits<Emits>()
</script>
```

#### Template Conventions
```vue
<template>
  <!-- Use semantic HTML -->
  <div class="log-entry" @click="emit('click', log)">
    <!-- Conditional rendering -->
    <span v-if="props.showTimestamp" class="timestamp">
      {{ formatTimestamp(log.timestamp) }}
    </span>

    <!-- List rendering with :key -->
    <div v-for="field in log.fields" :key="field.name">
      {{ field.name }}: {{ field.value }}
    </div>

    <!-- Two-way binding -->
    <input v-model="searchText" @input="handleSearch" />
  </div>
</template>
```

#### API Client Pattern
```typescript
// utils/api.ts - Fetch-based client
export async function getContainers(): Promise<Container[]> {
  const response = await fetch('/api/containers')
  if (!response.ok) {
    throw new Error(`HTTP error: ${response.status}`)
  }
  return response.json()
}

// Usage in component
const containers = ref<Container[]>([])

onMounted(async () => {
  try {
    containers.value = await getContainers()
  } catch (error) {
    console.error('Failed to load containers:', error)
  }
})
```

### Log Parser Conventions

#### ANSI Escape Code Handling
```go
// ANSI codes are used for boundary detection BEFORE stripping
func IsLikelyNewLogEntry(line string) bool {
    // 1. Check for ANSI codes at start (common in new entries)
    if startsWithANSI(line) {
        return true
    }

    // 2. Check for timestamps near beginning
    if hasTimestampPrefix(line) {
        return true
    }

    // 3. Check for log levels at start
    if hasLogLevelPrefix(line) {
        return true
    }

    // 4. Reject lines with leading whitespace (continuations)
    if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
        return false
    }

    return false
}
```

#### Key=Value Parsing Rules
```go
// Custom parser supporting:
// - Quoted strings: key="value with spaces"
// - Nested braces: data={field1: value1, field2: value2}
// - Escapes: key="value with \"quotes\""
// - Dotted keys: db.error, db.table
// - First key= determines message boundary
```

#### Multi-format Support
The parser handles multiple formats in order:
1. ANSI boundary detection (before stripping)
2. Key=value structured logs
3. JSON objects (single-line or formatted)
4. Timestamp extraction
5. Log level detection (DBG, TRC, INF, WRN, ERR, FATAL)
6. File location parsing (e.g., `pkg/handlers/stripe.go:85`)
7. ANSI color stripping (after boundary detection)

---

## Testing Strategy

### Unit Tests

**Location Pattern**: `*_test.go` files alongside source files

**Examples**:
```bash
pkg/logs/parser_test.go          # Log parsing tests
pkg/logs/docker_test.go           # Docker client tests
pkg/logstore/logstore_test.go     # Storage tests
pkg/sqlexplain/analyzer_test.go   # SQL analysis tests
pkg/httputil/httputil_test.go     # HTTP utility tests
pkg/sqlutil/sqlutil_test.go       # SQL utility tests
```

**Running Tests**:
```bash
go test ./...                     # All tests
go test ./pkg/logs                # Package-specific
go test -v ./pkg/logs             # Verbose output
go test -run TestParseLogLine     # Specific test
```

### Integration Tests

**Location**:
- `cmd/viewer/integration_test.go`
- `cmd/analyze/integration_test.go`

**Purpose**:
- Test Docker client integration
- Test database operations
- Test full request/response cycles

**Database Testing**:
```go
// Use in-memory SQLite for tests
func setupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }

    // Run migrations
    store.RunMigrations(db)

    return db
}
```

### Manual Testing Checklist

**Log Viewer**:
- [ ] Multiple containers running (use Docker Compose)
- [ ] Various log formats (structured, JSON, plain)
- [ ] ANSI color codes in logs
- [ ] Container start/stop events
- [ ] Filter by container, log level, text search
- [ ] Click trace IDs to activate SQL analyzer
- [ ] WebSocket reconnection on disconnect

**Request Manager**:
- [ ] Save GraphQL/REST requests
- [ ] Execute requests and capture logs
- [ ] View SQL queries extracted from logs
- [ ] Compare two executions
- [ ] Export data to ZIP

**SQL Analyzer**:
- [ ] N+1 query detection (>5 executions)
- [ ] Slow query identification
- [ ] EXPLAIN plan visualization (requires PostgreSQL)
- [ ] Index recommendations

---

## Common Tasks

### Task 1: Add a New Filter Type

**Example**: Add filtering by container port

**Steps**:

1. **Add reactive state** in Vue component:
```typescript
// src/views/LogsView.vue
const filters = reactive({
  containerPort: '',
  // ... existing filters
})
```

2. **Update filter logic**:
```typescript
const filteredLogs = computed(() => {
  return logs.value.filter(log => {
    // Container port filter
    if (filters.containerPort && !log.container.ports.includes(filters.containerPort)) {
      return false
    }
    // ... other filters
    return true
  })
})
```

3. **Add UI controls**:
```vue
<template>
  <input
    v-model="filters.containerPort"
    placeholder="Filter by port..."
    @input="applyFilters"
  />
</template>
```

### Task 2: Parse a New Log Format

**Example**: Add support for Logrus JSON format

**Steps**:

1. **Update parser** in `pkg/logs/parser.go`:
```go
func ParseLogLine(line string) (*LogEntry, error) {
    entry := &LogEntry{Raw: line}

    // Try parsing as Logrus JSON
    if strings.HasPrefix(line, "{") && strings.Contains(line, `"level"`) {
        var logrusData map[string]interface{}
        if err := json.Unmarshal([]byte(line), &logrusData); err == nil {
            entry.Message = logrusData["msg"].(string)
            entry.Level = logrusData["level"].(string)
            entry.Timestamp = logrusData["time"].(string)
            // ... extract other fields
            return entry, nil
        }
    }

    // ... existing parsers
}
```

2. **Add tests** in `pkg/logs/parser_test.go`:
```go
func TestParseLogrusJSON(t *testing.T) {
    line := `{"level":"info","msg":"Test message","time":"2024-01-23T10:00:00Z"}`
    entry, err := ParseLogLine(line)

    assert.NoError(t, err)
    assert.Equal(t, "info", entry.Level)
    assert.Equal(t, "Test message", entry.Message)
}
```

3. **Test manually** with real logs

### Task 3: Add a WebSocket Message Type

**Example**: Broadcast container stats

**Steps**:

1. **Define message struct** in `cmd/viewer/main.go`:
```go
type ContainerStatsMessage struct {
    Type  string            `json:"type"`  // "stats"
    Stats map[string]Stats  `json:"stats"`
}

type Stats struct {
    CPUPercent    float64 `json:"cpu_percent"`
    MemoryUsage   uint64  `json:"memory_usage"`
}
```

2. **Add broadcast function**:
```go
func broadcastContainerStats(stats map[string]Stats) {
    clientsMu.RLock()
    defer clientsMu.RUnlock()

    msg := ContainerStatsMessage{
        Type:  "stats",
        Stats: stats,
    }

    data, _ := json.Marshal(msg)

    for _, client := range clients {
        client.Conn.WriteMessage(websocket.TextMessage, data)
    }
}
```

3. **Handle in Vue component**:
```typescript
// src/views/LogsView.vue
const containerStats = ref<Map<string, Stats>>(new Map())

ws.onmessage = (event) => {
  const data = JSON.parse(event.data)

  if (data.type === 'stats') {
    containerStats.value = new Map(Object.entries(data.stats))
  }
}
```

4. **Display in template**:
```vue
<template>
  <div v-for="[id, stats] in containerStats" :key="id">
    CPU: {{ stats.cpu_percent }}%
    Memory: {{ formatBytes(stats.memory_usage) }}
  </div>
</template>
```

### Task 4: Add a Database Migration

**Example**: Add a new column to `executed_requests` table

**Steps**:

1. **Create migration file** in `pkg/store/migrations/`:
```bash
# Files are numbered sequentially: 001_*, 002_*, ..., 024_*
# Create 024_add_duration_column.sql
```

2. **Write SQL**:
```sql
-- +goose Up
ALTER TABLE executed_requests ADD COLUMN average_duration INTEGER DEFAULT 0;

-- +goose Down
ALTER TABLE executed_requests DROP COLUMN average_duration;
```

3. **Update GORM model** in `pkg/store/store.go`:
```go
type ExecutedRequest struct {
    ID              uint      `gorm:"primaryKey"`
    // ... existing fields
    AverageDuration int       `json:"average_duration"`
}
```

4. **Test migration**:
```bash
# Migrations run automatically on app start
./docker-log-viewer

# Or test explicitly
go test ./pkg/store
```

### Task 5: Add a New API Endpoint

**Example**: GET /api/containers/:id/stats

**Steps**:

1. **Add handler** in `pkg/controller/`:
```go
// pkg/controller/containers.go
func (c *Controller) GetContainerStats(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    containerID := vars["id"]

    stats, err := c.dockerClient.GetStats(containerID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}
```

2. **Register route** in `cmd/viewer/main.go`:
```go
// In setupRoutes()
r.HandleFunc("/api/containers/{id}/stats", controller.GetContainerStats).Methods("GET")
```

3. **Add TypeScript type** in `web/src/types/index.ts`:
```typescript
export interface ContainerStats {
  cpu_percent: number
  memory_usage: number
  network_rx_bytes: number
  network_tx_bytes: number
}
```

4. **Add API client method** in `web/src/utils/api.ts`:
```typescript
export async function getContainerStats(id: string): Promise<ContainerStats> {
  const response = await fetch(`/api/containers/${id}/stats`)
  if (!response.ok) {
    throw new Error(`Failed to fetch stats: ${response.statusText}`)
  }
  return response.json()
}
```

5. **Use in component**:
```typescript
const stats = ref<ContainerStats | null>(null)

async function loadStats(containerId: string) {
  try {
    stats.value = await getContainerStats(containerId)
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}
```

### Task 6: Add Code to an Existing Handler

**Location**: Most handlers are in `pkg/controller/` or `cmd/viewer/main.go`

**Pattern**:
1. Read the existing handler code first
2. Understand the data flow and validation
3. Add your logic following the same patterns
4. Update tests if applicable

**Handler Organization in cmd/viewer/main.go**:
- Section 1: Container & Log Management
- Section 2: SQL Analysis & Trace Management
- Section 3: Request & Execution Management
- Section 4: Server & Database Configuration
- Section 5: Container Retention Settings

---

## File Location Reference

### Finding Specific Functionality

**Container Management**:
- Docker client: `pkg/logs/docker.go`
- Container listing: `pkg/controller/containers.go`
- Container retention: `cmd/viewer/main.go` (Section 5)

**Log Parsing & Storage**:
- Log parser: `pkg/logs/parser.go`
- Timestamp parsing: `pkg/logs/timestamp.go`
- Log storage: `pkg/logstore/logstore.go`
- Log display: `web/src/components/LogStream.vue`

**SQL Analysis**:
- Query extraction: `pkg/sqlutil/sqlutil.go`
- EXPLAIN functionality: `pkg/sqlexplain/explain.go`
- Query comparison: `pkg/sqlexplain/analyzer.go`
- Index recommendations: `pkg/sqlexplain/index_analyzer.go`
- SQL detail view: `web/src/views/SqlDetailView.vue`

**Request Management**:
- Request CRUD: `pkg/controller/requests.go`
- Request execution: `cmd/graphql-tester/main.go`
- Request comparison: `cmd/analyze/main.go`
- Request UI: `web/src/views/RequestsView.vue`
- Request detail: `web/src/views/RequestDetailView.vue`

**WebSocket**:
- Server: `cmd/viewer/main.go` (handleWebSocket function)
- Client: Each Vue view has its own WebSocket logic

**Database Models**:
- GORM models: `pkg/store/store.go`
- Migrations: `pkg/store/migrations/*.sql`

**HTTP Utilities**:
- Request ID generation: `pkg/httputil/httputil.go`
- HTTP POST: `pkg/httputil/httputil.go`
- Log collection: `pkg/httputil/httputil.go`

**Configuration**:
- Backend config: `cmd/viewer/main.go` (environment variables)
- Frontend config: `web/vite.config.ts`
- Docker Compose: `docker-compose.yml`
- Air hot reload: `.air.toml`

---

## Key Constraints & Limitations

### Performance Limits

**Log Storage**:
- Maximum 10,000 logs in memory (configurable per container)
- Log lines >100KB are skipped
- UI displays last 1,000 filtered logs

**WebSocket**:
- 1-second message batching delay
- All clients receive filtered broadcasts
- No message persistence (logs lost on disconnect)

**Container Monitoring**:
- 5-second polling interval for container discovery
- Docker socket must be accessible
- Requires `/var/run/docker.sock` mount

### Technical Constraints

**EXPLAIN Plans**:
- Requires PostgreSQL connection (DATABASE_URL)
- Only works with PostgreSQL databases
- Cannot analyze other database types (MySQL, MongoDB, etc.)

**Docker Multiplexing**:
- Assumes standard streams: stdout(1), stderr(2)
- 8-byte header stripping
- May fail with custom Docker logging drivers

**Trace Filtering**:
- Requires exact match (no partial matching)
- Case-sensitive
- Must click on a trace ID field in logs

**Browser Support**:
- Modern browsers only (ES2020+)
- WebSocket support required
- JavaScript must be enabled

### Known Issues

**Log Parsing**:
- Multi-line logs may be split incorrectly without ANSI codes
- Double-encoded JSON requires manual normalization
- Some custom log formats may not parse correctly

**UI Performance**:
- Large number of logs can slow rendering
- No virtual scrolling (renders all 1,000 logs)
- Heavy filtering can be CPU-intensive

**Database**:
- SQLite has locking limitations (not suitable for high concurrency)
- Migrations run automatically on startup (no rollback mechanism)
- Database file grows indefinitely (no automatic cleanup)

---

## Critical Development Notes

### Before Making Changes

1. **Always read existing code first** - Never propose changes without reading the relevant files
2. **Understand the context** - Read related files to understand how components interact
3. **Follow existing patterns** - Don't introduce new patterns unless necessary
4. **Test your changes** - Run tests and manually verify functionality
5. **Format your code** - Run `make format` before committing

### Anti-Patterns to Avoid

**DON'T**:
- ❌ Create duplicate utility functions (use `pkg/httputil` and `pkg/sqlutil`)
- ❌ Manually manipulate DOM in Vue components (use templates)
- ❌ Add features not explicitly requested
- ❌ Over-engineer solutions
- ❌ Skip error handling
- ❌ Ignore existing conventions
- ❌ Commit without running `make format-check`

**DO**:
- ✅ Reuse existing utilities
- ✅ Follow Vue Composition API patterns
- ✅ Keep handlers focused and single-purpose
- ✅ Write tests for new functionality
- ✅ Use TypeScript types consistently
- ✅ Add comments for complex logic
- ✅ Check `REFACTORING_GUIDE.md` for recent changes

### Error Handling Patterns

**Go**:
```go
// Return errors, don't panic
func DoSomething() error {
    if err := validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    return nil
}

// HTTP handlers return appropriate status codes
func Handler(w http.ResponseWriter, r *http.Request) {
    if err := process(); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

**TypeScript**:
```typescript
// Use try/catch for async operations
try {
  const data = await fetchData()
  processData(data)
} catch (error) {
  console.error('Failed to fetch:', error)
  // Show user-friendly error message
}
```

### Security Considerations

**Input Validation**:
- Validate all user input
- Sanitize data before database insertion
- Escape HTML in log display (Vue handles automatically)

**SQL Injection Prevention**:
- Use GORM parameterized queries (automatic)
- Never concatenate user input into SQL
- Use `sqlutil.SubstituteVariables()` for safe substitution

**WebSocket Security**:
- Currently no authentication (local development tool)
- Docker socket access requires host permissions
- No rate limiting on WebSocket messages

---

## Additional Resources

### Related Documentation

- **README.md** - User-facing documentation
- **AGENTS.md** - Developer guide (older, less detailed)
- **REFACTORING_GUIDE.md** - Recent architectural changes
- **.github/copilot-instructions.md** - GitHub Copilot guide
- **cmd/analyze/README.md** - Query analysis tool docs
- **pkg/sqlexplain/README.md** - SQL EXPLAIN analyzers

### External Dependencies

- [Vue 3 Documentation](https://vuejs.org/)
- [Vite Documentation](https://vitejs.dev/)
- [GORM Documentation](https://gorm.io/)
- [Gorilla Mux](https://github.com/gorilla/mux)
- [Docker SDK for Go](https://pkg.go.dev/github.com/docker/docker)

### Useful Commands Reference

```bash
# Development
docker-compose up              # Start with hot reload
./docker-log-viewer            # Run locally
make format                    # Format all code
make test                      # Run all tests

# Building
./build.sh                     # Build everything
go build -o viewer cmd/viewer/main.go

# Frontend
cd web
npm install                    # Install dependencies
npm run dev                    # Dev server (port 5173)
npm run build                  # Production build
npm run type-check             # TypeScript validation

# Testing
go test ./...                  # All Go tests
go test -v ./pkg/logs          # Verbose package test
cd web && npm run lint         # Lint JavaScript

# Database
# Migrations run automatically on startup
# Database file: graphql-requests.db
```

---

## Version History

- **v1.0** (2026-01-23) - Initial comprehensive AI assistant guide
  - Documented codebase structure
  - Added development workflows
  - Included code conventions
  - Detailed common tasks
  - Added file location reference

---

**Last Updated**: 2026-01-23
**Maintained By**: AI assistants working on this codebase
**Purpose**: Provide comprehensive context for effective AI-assisted development
