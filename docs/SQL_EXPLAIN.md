# SQL EXPLAIN Tool

This tool allows you to run PostgreSQL EXPLAIN plans on SQL queries extracted from Docker logs.

## Setup

### Database Connection

The EXPLAIN feature requires a PostgreSQL database connection. Set the `DATABASE_URL` environment variable:

```bash
export DATABASE_URL="host=localhost port=5432 user=postgres password=postgres dbname=mydb sslmode=disable"
```

Or use the default connection string (localhost PostgreSQL with user/pass `postgres/postgres`).

### Running the Application

```bash
# Build the application
go build -o docker-log-parser

# Set database connection (optional)
export DATABASE_URL="postgresql://user:password@localhost:5432/database?sslmode=disable"

# Run the application
./docker-log-parser
```

The application will start on `http://localhost:9000`. If the database connection fails, the EXPLAIN feature will be disabled but the log viewer will continue to work normally.

## Usage

1. **Filter by trace/request/span ID**: Click on any trace_id, request_id, or span_id value in the logs
2. **View SQL Analyzer**: The SQL Query Analyzer panel will open on the right showing:
   - Overview statistics
   - Slowest queries
   - Most frequent queries
   - Potential N+1 issues
   - Tables accessed

3. **Run EXPLAIN**: Click the "Run EXPLAIN" button on any query to see its execution plan

### EXPLAIN Results

The EXPLAIN modal shows:
- The SQL query (with variables substituted if available)
- A formatted execution plan with:
  - Node types (Sequential Scan, Index Scan, etc.)
  - Cost estimates
  - Row estimates
  - Index conditions and filters
  - Join types

### Variable Substitution

The tool automatically substitutes PostgreSQL placeholders ($1, $2, etc.) with actual values from the `db.vars` field in your logs. If a log entry includes `db.vars=["value1", "value2"]`, these values will be used to replace $1, $2, etc. in the query before running EXPLAIN.

## Features

- **EXPLAIN ANALYZE**: Uses `EXPLAIN (ANALYZE, FORMAT JSON)` to get actual execution metrics including real timing data
- **Visual Plan**: Hierarchical display of the query execution plan
- **Cost Analysis**: Shows startup and total cost estimates
- **Row Estimates**: Displays expected row counts at each node
- **Index Information**: Highlights index usage and conditions

## Example

Given a log entry like:
```
[sql]: SELECT * FROM users WHERE id = $1 AND active = $2
db.operation=select db.rows=1 db.table=users duration=1.23
```

Clicking "Run EXPLAIN" will:
1. Connect to the database
2. Run `EXPLAIN (ANALYZE, FORMAT JSON) SELECT * FROM users WHERE id = $1 AND active = $2`
3. Display the execution plan with cost and row estimates

## Troubleshooting

### "Database connection not configured"
- Ensure the `DATABASE_URL` environment variable is set correctly
- Check that PostgreSQL is running and accessible
- Verify database credentials

### "Error running EXPLAIN"
- The query may have syntax errors
- The tables referenced may not exist in the database
- Missing permissions for the database user

### "Unable to format plan"
- The EXPLAIN output format may not be recognized
- Check the application logs for detailed error messages

## Development

### Testing

Run the test suite:
```bash
go test -v
```

Test variable substitution:
```bash
go test -v -run TestSubstituteVariables
```

## Security Considerations

- The tool uses `EXPLAIN` without `ANALYZE` to prevent query execution
- Variable values are properly escaped to prevent SQL injection
- Database credentials should be stored securely (use environment variables)
- Consider restricting database user permissions to read-only access
