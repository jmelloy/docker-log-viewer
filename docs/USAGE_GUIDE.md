# SQL EXPLAIN Feature - Usage Guide

## Overview

The SQL EXPLAIN tool allows you to analyze the execution plans of SQL queries extracted from your Docker logs. This is particularly useful for:

- Understanding query performance
- Identifying missing indexes
- Detecting inefficient query patterns
- Optimizing database access

## Visual Guide

### 1. Main Interface

![Docker Log Viewer UI](https://github.com/user-attachments/assets/57eb6fde-ff49-499d-a468-f47479c3c723)

The main interface shows:
- **Left Sidebar**: Container filtering and log level selection
- **Center**: Live log display
- **Right Panel**: SQL Query Analyzer (appears when filtering by trace/request/span ID)

### 2. Activating the SQL Analyzer

To activate the SQL Query Analyzer:

1. Click on any `request_id`, `span_id`, or `trace_id` value in the logs
2. The right panel will slide in showing query statistics
3. You'll see sections for:
   - Overview (total queries, unique queries, duration stats)
   - Slowest Queries
   - Most Frequent Queries
   - Potential N+1 Issues
   - Tables Accessed

### 3. Running EXPLAIN

Each query in the analyzer panel has a **"Run EXPLAIN"** button:

```
┌─────────────────────────────────────────────┐
│ Slowest Queries                             │
├─────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────┐ │
│ │ 12.45ms                                 │ │
│ │ SELECT * FROM users WHERE id = $1       │ │
│ │ Table: users | Op: select | Rows: 1     │ │
│ │ [Run EXPLAIN]                           │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
```

Click the **"Run EXPLAIN"** button to see the execution plan.

### 4. EXPLAIN Results Modal

The EXPLAIN modal displays:

**Query Section:**
```sql
SELECT * FROM users WHERE id = 123
```

**Execution Plan:**
```
Index Scan using users_pkey on users
  cost=0.15..8.17 rows=1
  Index Cond: (id = '123'::text)
```

The plan shows:
- **Node Type**: The operation type (Index Scan, Sequential Scan, etc.)
- **Cost**: Startup cost..Total cost (lower is better)
- **Rows**: Expected number of rows returned
- **Conditions**: Index conditions and filters

### 5. Example Execution Plans

#### Good: Index Scan
```
Index Scan using idx_users_id on users
  cost=0.15..8.17 rows=1
  Index Cond: (id = $1)
```
✅ Using an index - efficient query

#### Bad: Sequential Scan
```
Seq Scan on users
  cost=0.00..15234.00 rows=1
  Filter: (id = $1)
```
❌ No index used - scans entire table

#### Complex: Join with Hash
```
Hash Join
  cost=234.50..1256.73 rows=100
  Hash Cond: (messages.user_id = users.id)
  -> Seq Scan on messages
       cost=0.00..856.00 rows=50000
  -> Hash
       cost=156.00..156.00 rows=10000
       -> Seq Scan on users
            cost=0.00..156.00 rows=10000
```

## Step-by-Step Example

### Setup

1. Start PostgreSQL:
```bash
docker-compose -f docker-compose.example.yml up -d
```

2. Start the log viewer with database connection:
```bash
export DATABASE_URL="postgresql://postgres:postgres@localhost:5432/testdb"
./docker-log-viewer
```

3. Open browser to http://localhost:9000

### Workflow

1. **View logs** - Watch your application logs streaming in
2. **Filter by trace** - Click a `request_id` to see all logs for that request
3. **Analyze queries** - Review the SQL queries in the analyzer panel
4. **Run EXPLAIN** - Click "Run EXPLAIN" on a slow query
5. **Optimize** - Use the plan to identify missing indexes or query issues

### Example: Finding a Missing Index

**Scenario**: You notice a slow query in the logs:

```
SELECT * FROM messages WHERE user_id = $1 AND deleted_at IS NULL
db.operation=select db.rows=1000 duration=125.43ms
```

**Steps**:

1. Click "Run EXPLAIN" on this query
2. See the execution plan:
   ```
   Seq Scan on messages
     cost=0.00..15234.00 rows=1000
     Filter: (user_id = $1 AND deleted_at IS NULL)
   ```

3. Notice it's using a Sequential Scan - no index!
4. Create an index:
   ```sql
   CREATE INDEX idx_messages_user_deleted 
   ON messages(user_id) 
   WHERE deleted_at IS NULL;
   ```

5. Run EXPLAIN again to verify:
   ```
   Index Scan using idx_messages_user_deleted on messages
     cost=0.29..45.32 rows=1000
     Index Cond: (user_id = $1)
   ```

6. Much better! Cost reduced from 15234 to 45.32

## Understanding EXPLAIN Output

### Node Types

- **Seq Scan**: Full table scan (slow for large tables)
- **Index Scan**: Using an index to find rows (usually good)
- **Index Only Scan**: Using index without accessing table (very good)
- **Bitmap Heap Scan**: Using index bitmap (good for many rows)
- **Hash Join**: Building hash table for join (common)
- **Nested Loop**: Joining by nested iteration (can be slow)
- **Merge Join**: Sorted merge join (efficient for sorted data)

### Cost Interpretation

Format: `cost=startup_cost..total_cost`

- **Startup Cost**: Cost before first row can be returned
- **Total Cost**: Cost to return all rows
- Lower is better
- Arbitrary units (not time)
- Compare relative costs between queries

### Row Estimates

- `rows=N`: Expected number of rows
- Based on table statistics
- May not match actual results
- Use `ANALYZE` to update statistics

## Tips and Best Practices

### 1. Use EXPLAIN Regularly

- Run EXPLAIN on slow queries (>10ms)
- Compare before/after optimization
- Check new queries in development

### 2. Look for Red Flags

❌ Sequential scans on large tables
❌ Nested loops with many rows
❌ Missing indexes on join/filter columns
❌ High cost estimates

### 3. Optimize Based on Plans

✅ Add indexes for frequently filtered columns
✅ Use partial indexes for common WHERE clauses
✅ Consider covering indexes for SELECT columns
✅ Update table statistics with ANALYZE

### 4. Database Connection Security

- Use read-only database user for EXPLAIN
- Limit network access to database
- Store DATABASE_URL in environment, not code
- Consider using connection pooling

## Troubleshooting

### Error: "Database connection not configured"

**Solution**: Set the DATABASE_URL environment variable:
```bash
export DATABASE_URL="postgresql://user:pass@host:5432/db"
```

### Error: "Error running EXPLAIN: syntax error"

**Possible causes**:
- Query has placeholder variables that couldn't be substituted
- Query is incomplete/malformed in logs
- Database doesn't support the SQL syntax

**Solution**: Check the query in the modal, it may need manual adjustment

### Error: "relation does not exist"

**Possible causes**:
- Table doesn't exist in the connected database
- Using wrong database/schema
- Table name case sensitivity

**Solution**: Ensure you're connected to the correct database with the schema that matches your logs

### EXPLAIN shows different results than expected

**Possible causes**:
- Database statistics are outdated
- Different data volume than production
- Using test database instead of production

**Solution**: Run `ANALYZE` on your tables or use a production replica

## Advanced Usage

### Custom Variable Substitution

If your logs include a `db.vars` field with query parameters, you can extend the feature to use them:

1. Update `extractSQLQueries` in `web/app.js` to parse `db.vars`
2. Pass variables to the EXPLAIN endpoint
3. The backend will substitute them in the query

Example log format:
```
[sql]: SELECT * FROM users WHERE id = $1 AND name = $2
db.vars=["123", "John"]
```

### Comparing Multiple Queries

While not currently implemented, you could extend this to:
- Store multiple EXPLAIN results
- Compare execution plans side-by-side
- Track performance changes over time

## FAQ

**Q: Will EXPLAIN execute my query?**
A: No, we use `EXPLAIN` without `ANALYZE`, which only analyzes the query plan without executing it.

**Q: Can I use this with other databases besides PostgreSQL?**
A: Currently only PostgreSQL is supported. MySQL and other databases use different EXPLAIN formats.

**Q: Does this work with read replicas?**
A: Yes, since EXPLAIN doesn't modify data, it works perfectly with read-only replicas.

**Q: How accurate are the cost estimates?**
A: They're relative estimates based on table statistics. Actual performance may vary, but relative costs are useful for comparison.

**Q: Can I export EXPLAIN results?**
A: Not currently, but this would be a good feature addition. You can copy from the modal.

## Next Steps

- Read [SQL_EXPLAIN.md](SQL_EXPLAIN.md) for technical details
- Try the example setup with docker-compose
- Experiment with different queries from your logs
- Share feedback and suggestions!
