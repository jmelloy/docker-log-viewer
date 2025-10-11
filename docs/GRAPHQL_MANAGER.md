# GraphQL Request Manager

A tool for managing and analyzing GraphQL/API requests with Docker log integration. Store requests, execute them, and analyze SQL query performance with before/after comparison capabilities.

## Features

- **Request Library**: Save and organize GraphQL/API requests
- **Execution Tracking**: Track all request executions with full details
- **Docker Log Integration**: Automatically capture logs for each request
- **SQL Query Analysis**: Extract and analyze SQL queries from logs
- **Web UI**: Modern interface for managing requests and viewing results
- **CLI Tool**: Command-line interface for automation

## Database Schema

The system uses SQLite to store:
- **Requests**: Saved GraphQL/API request templates
- **Executions**: Each time a request is executed
- **Logs**: Docker container logs associated with each execution
- **SQL Queries**: Extracted SQL queries with performance metrics

## CLI Tool: graphql-tester

### Usage

```bash
# Save a new request
./graphql-tester -url "https://api.example.com/graphql" \
                 -data graphql-operations-unique/AuthConfig.json \
                 -name "AuthConfig"

# Save and execute immediately
./graphql-tester -url "https://api.example.com/graphql" \
                 -data graphql-operations-unique/AuthConfig.json \
                 -execute

# List all saved requests
./graphql-tester -list

# Delete a request
./graphql-tester -delete 1

# With authentication
./graphql-tester -url "https://api.example.com/graphql" \
                 -data query.json \
                 -token "your-bearer-token" \
                 -dev-id "dev-user-id"
```

### Options

- `-db`: Database file path (default: `graphql-requests.db`)
- `-url`: GraphQL/API endpoint URL
- `-data`: JSON file with request data
- `-name`: Name for the request (defaults to filename)
- `-execute`: Execute the request immediately after saving
- `-list`: List all saved requests
- `-delete`: Delete request by ID
- `-token`: Bearer token for authentication
- `-dev-id`: X-GlueDev-UserID header value
- `-timeout`: Timeout for log collection (default: 10s)

## Web UI

Access the Request Manager at `http://localhost:9000/requests.html`

### Features

1. **Request Library**
   - View all saved requests
   - Create new requests
   - Delete requests

2. **Request Details**
   - View request configuration
   - See all executions
   - Execute request with one click

3. **Execution Analysis**
   - HTTP response details
   - Captured Docker logs
   - SQL query analysis
   - Performance metrics

4. **SQL Analysis**
   - Total and unique query counts
   - Average and total duration
   - Tables accessed
   - Query-by-query breakdown
   - N+1 detection

## API Endpoints

The viewer exposes these endpoints:

```
GET    /api/requests              - List all saved requests
POST   /api/requests              - Create a new request
GET    /api/requests/:id          - Get request details
DELETE /api/requests/:id          - Delete a request
POST   /api/requests/:id/execute  - Execute a request

GET    /api/executions?request_id=:id  - List executions for a request
GET    /api/executions/:id             - Get execution details with logs and SQL
```

## Before/After Analysis Workflow

1. **Baseline**: Execute a request before making changes
   ```bash
   ./graphql-tester -url "https://api.staging.com/graphql" \
                    -data operations/FetchUsers.json \
                    -execute
   ```

2. **Make Changes**: Deploy your code changes

3. **After**: Execute the same request again
   ```bash
   ./graphql-tester -url "https://api.staging.com/graphql" \
                    -data operations/FetchUsers.json \
                    -execute
   ```

4. **Compare**: Use the web UI to compare:
   - SQL query counts
   - Query durations
   - Tables accessed
   - N+1 query patterns
   - Log patterns

## Example: Bulk Import from graphql-operations

```bash
# Import all GraphQL operations
for file in graphql-operations-unique/*.json; do
  name=$(basename "$file" .json)
  ./graphql-tester -url "https://api.example.com/graphql" \
                   -data "$file" \
                   -name "$name"
done

# List imported requests
./graphql-tester -list
```

## SQL Query Analysis

The system automatically extracts SQL queries from logs that contain `[sql]:` markers. Each query is analyzed for:

- **Duration**: Execution time in milliseconds
- **Table**: Primary table accessed
- **Operation**: SELECT, INSERT, UPDATE, DELETE
- **Rows**: Number of rows affected
- **Normalized Query**: Queries with parameters replaced for grouping

### N+1 Detection

Queries executed more than 5 times in a single request are flagged as potential N+1 issues.

## Log Collection

Logs are collected using the Docker API by:
1. Streaming logs from all running containers
2. Matching logs to requests via `request_id`, `requestId`, or similar fields
3. Collecting logs for a configurable timeout (default 10 seconds)
4. Storing logs with execution for later analysis

## Database Location

By default, the database is stored at `./graphql-requests.db`. You can change this with the `-db` flag.

The database includes:
- Foreign key constraints for data integrity
- Cascade deletes (deleting a request removes all executions)
- Indexes for query performance

## Integration with Existing Tools

This tool is designed to work alongside:
- **compare tool**: For side-by-side URL comparisons
- **log viewer**: For real-time log monitoring
- **SQL EXPLAIN**: For query plan analysis

## Environment Variables

- `BEARER_TOKEN`: Default bearer token for authentication
- `X_GLUE_DEV_USER_ID`: Default dev user ID header

## Tips

1. **Organizing Requests**: Use descriptive names that indicate the operation and use case
2. **Log Timeout**: Increase timeout for slow endpoints: `-timeout 30s`
3. **Batch Execution**: Use the web UI to quickly re-execute multiple requests
4. **SQL Analysis**: Look for queries with high counts as optimization opportunities
5. **Before/After**: Keep request names consistent for easier comparison

## Troubleshooting

**No logs captured**: 
- Ensure Docker containers are running and logging
- Verify logs include a `request_id` field
- Increase the timeout with `-timeout`

**SQL queries not detected**:
- Logs must include `[sql]:` marker
- Check that `db.table`, `db.operation`, `duration` fields are present

**Request execution fails**:
- Check URL is accessible
- Verify authentication tokens
- Review error in execution details
