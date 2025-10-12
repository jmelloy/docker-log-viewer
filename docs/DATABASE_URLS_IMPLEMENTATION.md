# Database URLs Feature Implementation

## Overview
This implementation adds support for managing multiple database connection strings for EXPLAIN plan generation. Previously, the system only supported a single database connection via the `DATABASE_URL` environment variable. Now, multiple database connections can be configured and associated with specific servers.

## Changes Made

### 1. Database Schema

#### New Table: `database_urls`
```sql
CREATE TABLE database_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    connection_string TEXT NOT NULL,
    database_type TEXT NOT NULL DEFAULT 'postgresql',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);
```

#### Updated Table: `servers`
Added `default_database_id` field to link servers with their default database for EXPLAIN queries:
```sql
ALTER TABLE servers ADD COLUMN default_database_id INTEGER;
```

### 2. Go Models (pkg/store/store.go)

#### New Model: DatabaseURL
```go
type DatabaseURL struct {
    ID               uint
    Name             string
    ConnectionString string
    DatabaseType     string
    CreatedAt        time.Time
    UpdatedAt        time.Time
    DeletedAt        gorm.DeletedAt
}
```

#### Updated Model: Server
```go
type Server struct {
    ID                uint
    Name              string
    URL               string
    BearerToken       string
    DevID             string
    DefaultDatabaseID *uint
    DefaultDatabase   *DatabaseURL  // Preloaded relationship
    CreatedAt         time.Time
    UpdatedAt         time.Time
    DeletedAt         gorm.DeletedAt
}
```

### 3. CRUD Operations (pkg/store/store.go)

Added complete CRUD operations for DatabaseURL:
- `CreateDatabaseURL(dbURL *DatabaseURL) (int64, error)`
- `GetDatabaseURL(id int64) (*DatabaseURL, error)`
- `ListDatabaseURLs() ([]DatabaseURL, error)`
- `UpdateDatabaseURL(dbURL *DatabaseURL) error`
- `DeleteDatabaseURL(id int64) error`

Updated Server operations to preload DefaultDatabase:
- `GetServer()` - Now preloads the DefaultDatabase relationship
- `ListServers()` - Now preloads the DefaultDatabase relationship

### 4. SQL Explain Enhancement (pkg/sqlexplain/explain.go)

Updated `Request` struct to accept an optional connection string:
```go
type Request struct {
    Query            string
    Variables        map[string]string
    ConnectionString string  // Optional: use specific connection instead of default
}
```

Updated `Explain()` function:
- If `ConnectionString` is provided in the request, it creates a temporary connection for that query
- Otherwise, it falls back to the default DATABASE_URL connection
- Properly closes temporary connections after use

### 5. API Endpoints (cmd/viewer/main.go)

Added new REST endpoints:
- `GET /api/database-urls` - List all database URLs
- `POST /api/database-urls` - Create a new database URL

Example POST request:
```json
{
  "name": "Production DB",
  "connectionString": "postgresql://user:pass@localhost:5432/prod",
  "databaseType": "postgresql"
}
```

### 6. Tests (pkg/store/store_test.go)

Added comprehensive test `TestDatabaseURL` that verifies:
- Creating database URLs
- Retrieving database URLs
- Listing database URLs
- Updating database URLs
- Creating servers with default database
- Preloading default database relationships
- Deleting database URLs

## Usage Examples

### 1. Create a Database URL via API
```bash
curl -X POST http://localhost:9000/api/database-urls \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production DB",
    "connectionString": "postgresql://user:pass@localhost:5432/prod",
    "databaseType": "postgresql"
  }'
```

### 2. List Database URLs
```bash
curl http://localhost:9000/api/database-urls
```

### 3. Create Server with Default Database
```go
dbID := uint(1)
server := &store.Server{
    Name:              "API Server",
    URL:               "https://api.example.com/graphql",
    DefaultDatabaseID: &dbID,
}
```

### 4. Run EXPLAIN with Custom Connection
```go
req := sqlexplain.Request{
    Query:            "SELECT * FROM users WHERE id = $1",
    ConnectionString: "postgresql://user:pass@localhost:5432/db",
    Variables:        map[string]string{"1": "123"},
}
resp := sqlexplain.Explain(req)
```

## Migration Path

1. Existing databases will automatically run migrations on startup
2. Migration `00011_create_database_urls.sql` creates the new table
3. Migration `00012_add_default_database_id_to_servers.sql` adds the field to servers
4. Existing servers will have `default_database_id` = NULL (optional field)
5. No data migration required - fully backward compatible

## Benefits

1. **Multi-Database Support**: Different servers can use different databases for EXPLAIN queries
2. **Flexibility**: Can override database connection per-request via API
3. **Backward Compatibility**: Existing functionality with DATABASE_URL still works
4. **Security**: Connection strings stored in database (not in code)
5. **Centralized Management**: Database URLs can be managed via API

## Future Enhancements

Potential improvements for the future:
- Add UI for managing database URLs in the web interface
- Support for other database types beyond PostgreSQL
- Connection pooling for frequently-used databases
- Database URL validation before saving
- Encryption of connection strings at rest
