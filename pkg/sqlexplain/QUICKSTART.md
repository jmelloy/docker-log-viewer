# Quick Start Guide: Explain Plan Analyzers

## Installation

The analyzers are part of the `pkg/sqlexplain` package and are ready to use.

```bash
go get docker-log-parser/pkg/sqlexplain
```

## Quick Examples

### 1. Compare Two Query Sets

**Goal**: Identify query and performance differences between two test runs.

```go
import "docker-log-parser/pkg/sqlexplain"

// Set 1: Production environment
set1 := []sqlexplain.QueryWithPlan{
    {
        Query: "SELECT * FROM users WHERE email = $1",
        NormalizedQuery: "SELECT * FROM users WHERE email = $N",
        OperationName: "GetUser",
        Timestamp: 1000,
        DurationMS: 125.5,
        ExplainPlan: `[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 150.0}}]`,
    },
}

// Set 2: After adding index
set2 := []sqlexplain.QueryWithPlan{
    {
        Query: "SELECT * FROM users WHERE email = $1",
        NormalizedQuery: "SELECT * FROM users WHERE email = $N",
        OperationName: "GetUser",
        Timestamp: 1000,
        DurationMS: 25.5,
        ExplainPlan: `[{"Plan": {"Node Type": "Index Scan", "Total Cost": 45.0}}]`,
    },
}

comparison := sqlexplain.CompareQuerySets(set1, set2)

// Check results
fmt.Printf("Performance improved by %.1f%%\n", 
    comparison.CommonQueries[0].DurationDiffPct)
```

**Output:**
```
Performance improved by -79.8%
```

### 2. Get Index Recommendations

**Goal**: Find missing indexes that could improve performance.

```go
import "docker-log-parser/pkg/sqlexplain"

queries := []sqlexplain.QueryWithPlan{
    {
        Query: "SELECT * FROM users WHERE email = $1",
        NormalizedQuery: "SELECT * FROM users WHERE email = $N",
        DurationMS: 125.5,
        TableName: "users",
        ExplainPlan: `[{"Plan": {
            "Node Type": "Seq Scan",
            "Relation Name": "users",
            "Total Cost": 15234.00,
            "Plan Rows": 10000,
            "Filter": "(email = 'test@example.com'::text)"
        }}]`,
    },
}

analysis := sqlexplain.AnalyzeIndexUsage(queries)

// Print recommendations
for _, rec := range analysis.Recommendations {
    if rec.Priority == "high" {
        fmt.Printf("High Priority: %s\n", rec.SQLCommand)
    }
}
```

**Output:**
```
High Priority: CREATE INDEX idx_users_email ON users (email);
```

### 3. Using with cmd/compare Tool

**Goal**: Use the analyzers with existing compare tool queries.

```go
import (
    "docker-log-parser/cmd/compare"
    "docker-log-parser/pkg/sqlexplain"
)

// Your existing compare tool queries
queries := []compare.SQLQuery{
    {
        Query: "SELECT * FROM users WHERE id = $1",
        Normalized: "SELECT * FROM users WHERE id = $N",
        Duration: 50.0,
        Table: "users",
    },
}

// Convert and analyze
analysis := compare.AnalyzeIndexUsageForQueries(queries, "GetUser")

// Format output
fmt.Println(compare.FormatIndexRecommendations(analysis))
```

**Output:**
```
Index Recommendations (1 total, 0 high priority):

1. [MEDIUM] users
   Columns: id
   Reason: Sequential scan on 'users' filtering by id (occurred 1 times)
   Impact: Medium - Should improve query performance
   SQL: CREATE INDEX idx_users_id ON users (id);
   Affected Queries: 1
```

## Common Use Cases

### Finding Slow Queries

```go
analysis := sqlexplain.AnalyzeIndexUsage(queries)

for _, seqScan := range analysis.SequentialScans {
    if seqScan.DurationMS > 100 {
        fmt.Printf("Slow query on %s: %.2fms\n", 
            seqScan.TableName, seqScan.DurationMS)
    }
}
```

### Tracking Performance Over Time

```go
// Before optimization
baseline := sqlexplain.AnalyzeIndexUsage(queriesBefore)

// After optimization
current := sqlexplain.AnalyzeIndexUsage(queriesAfter)

improvement := baseline.Summary.AvgQueryCost - current.Summary.AvgQueryCost
fmt.Printf("Average cost reduced by %.2f\n", improvement)
```

### Detecting N+1 Queries

```go
analysis := sqlexplain.AnalyzeIndexUsage(queries)

for _, seqScan := range analysis.SequentialScans {
    if seqScan.Occurrences > 10 {
        fmt.Printf("Possible N+1: %s executed %d times\n",
            seqScan.NormalizedQuery, seqScan.Occurrences)
    }
}
```

## Understanding Output

### Comparison Summary

```go
comparison.Summary
    TotalQueriesSet1      // Total queries in first set
    TotalQueriesSet2      // Total queries in second set
    CommonQueries         // Queries in both sets
    QueriesWithPlanChange // Queries that changed execution plans
```

### Index Analysis Summary

```go
analysis.Summary
    TotalQueries         // All queries analyzed
    SequentialScans      // Number of seq scans detected
    IndexScans           // Number of index scans
    TotalRecommendations // All recommendations
    HighPriorityRecs     // High priority recommendations
```

### Priority Levels

- **High**: Frequent queries on large tables with high cost
- **Medium**: Moderate frequency or cost
- **Low**: Infrequent or low cost queries

## Tips

1. **Always collect EXPLAIN plans**: Analyzers need explain plan data for meaningful recommendations
2. **Sort by operation**: Group related queries together for better comparison
3. **Focus on high priority**: Start with high priority recommendations for best ROI
4. **Monitor after changes**: Re-run analysis after adding indexes to verify improvement
5. **Consider query frequency**: An index that benefits 1000 queries is more valuable than one that benefits 1 query

## Next Steps

- Read the [full documentation](README.md) for detailed API reference
- Check out the [tests](analyzer_test.go) for more examples
- Review the [source code](analyzer.go) to understand the algorithms
