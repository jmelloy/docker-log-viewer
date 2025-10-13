# Query Analysis Tool

Compare and analyze SQL queries from two request executions stored in the database.

## Usage

```bash
./bin/analyze -exec1 <id1> -exec2 <id2> [options]
```

## Options

- `-db string` - Path to SQLite database (default: "graphql-requests.db")
- `-exec1 int` - First execution ID (required)
- `-exec2 int` - Second execution ID (required)
- `-output string` - Output file path (optional, defaults to stdout)
- `-verbose` - Show detailed query lists for both executions

## Examples

### Basic comparison
Compare queries from two executions and output to console:
```bash
./bin/analyze -exec1 1 -exec2 2
```

### Save to file
Save the analysis report to a file:
```bash
./bin/analyze -exec1 1 -exec2 2 -output analysis-report.txt
```

### Verbose output
Include all queries in the report:
```bash
./bin/analyze -exec1 1 -exec2 2 -verbose
```

### Custom database
Use a specific database file:
```bash
./bin/analyze -db /path/to/custom.db -exec1 1 -exec2 2
```

## Report Contents

The analysis report includes:

### 1. Execution Details
- Request ID, duration, status code, and query count for each execution
- Request name and server (if available)

### 2. Query Comparison Summary
- Total query counts and average durations
- Unique query counts
- Common queries between executions
- Queries with execution plan changes

### 3. Performance Differences
- Queries that got faster or slower between executions
- Duration change percentages
- Query count changes

### 4. Unique Queries
- Queries only present in execution 1
- Queries only present in execution 2

### 5. Index Analysis
For each execution:
- Sequential scan issues
- Index scan counts
- Index recommendations with priority levels

### 6. Index Recommendations
- Suggested indexes to improve performance
- Expected impact estimates
- SQL commands to create indexes

### 7. Detailed Query Lists (Verbose Mode)
When `-verbose` flag is used:
- Complete list of all queries from both executions
- Duration, rows affected, and table information
- GraphQL operation names (if available)

## Use Cases

### Performance Regression Detection
Compare executions before and after a code change to identify performance regressions:
```bash
./bin/analyze -exec1 100 -exec2 101 -output before-after-comparison.txt
```

### Query Optimization Analysis
After adding indexes or optimizing queries, compare performance:
```bash
./bin/analyze -exec1 200 -exec2 201 -verbose
```

### A/B Testing
Compare two different approaches to the same operation:
```bash
./bin/analyze -exec1 50 -exec2 51 -output ab-test-results.txt
```

## Workflow

1. **Execute requests** using the GraphQL Tester or web UI to save execution data
2. **Note execution IDs** from the database or web UI
3. **Run analysis** to compare the two executions
4. **Review report** to identify differences and performance issues

## Related Tools

- **graphql-tester** - Execute and save requests
- **compare** - Compare two API endpoints in real-time
- **docker-log-viewer** - Web UI for viewing and managing executions

## Integration with Other Tools

This tool works with data stored by:
- The web-based request manager (`/requests.html`)
- The `graphql-tester` CLI tool
- Any execution tracked in the database

Execution IDs can be found in:
- The web UI at http://localhost:9000/requests.html
- By running `./bin/graphql-tester -list`
- Directly in the database (`executed_requests` table)
