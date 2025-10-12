# SQL Explain Plan and Index Analyzers

This package provides tools for analyzing SQL query execution plans and generating index recommendations.

## Features

### 1. Explain Plan Comparison (`analyzer.go`)

Compare SQL query execution plans between two sets of queries to identify:
- Which queries are unique to each set
- Common queries with performance differences
- Explain plan changes (e.g., seq scan → index scan)
- Cost and row estimate differences

### 2. Index Usage Analysis (`index_analyzer.go`)

Analyze query execution plans to:
- Detect sequential scans on large tables
- Track index usage statistics
- Generate prioritized index recommendations
- Identify potential N+1 query patterns

## Types

### QueryWithPlan

Represents a SQL query with its execution plan:

```go
type QueryWithPlan struct {
    Query            string  // The actual SQL query
    NormalizedQuery  string  // Query with normalized parameters
    OperationName    string  // GraphQL operation or grouping identifier
    Timestamp        int64   // Unix timestamp for ordering
    DurationMS       float64 // Query execution time
    TableName        string  // Primary table accessed
    Operation        string  // SQL operation type (select, insert, etc.)
    Rows             int     // Number of rows affected
    ExplainPlan      string  // JSON string of PostgreSQL EXPLAIN plan
    Variables        string  // Query variables/parameters
}
```

### ExplainPlanComparison

Result of comparing two query sets:

```go
type ExplainPlanComparison struct {
    QueriesOnlyInSet1      []QueryWithPlan       // Queries only in first set
    QueriesOnlyInSet2      []QueryWithPlan       // Queries only in second set
    CommonQueries          []QueryPlanComparison // Queries in both sets
    PlanDifferences        []QueryPlanComparison // Queries with plan changes
    PerformanceDifferences []QueryPlanComparison // Queries with >20% perf change
    Summary                ComparisonSummary     // High-level statistics
}
```

### IndexAnalysis

Result of analyzing queries for index usage:

```go
type IndexAnalysis struct {
    Recommendations []IndexRecommendation // Prioritized index recommendations
    SequentialScans []SequentialScanIssue // Queries doing seq scans
    IndexUsageStats []IndexUsageStat      // Which indexes are being used
    Summary         IndexAnalysisSummary  // High-level statistics
}
```

### IndexRecommendation

A suggestion to add or modify an index:

```go
type IndexRecommendation struct {
    TableName       string   // Table to add index to
    Columns         []string // Columns to include in index
    Reason          string   // Why this index is recommended
    EstimatedImpact string   // Expected performance improvement
    Priority        string   // "high", "medium", or "low"
    SQLCommand      string   // CREATE INDEX statement
    AffectedQueries int      // Number of queries that would benefit
}
```

## Usage Examples

### Comparing Query Sets

```go
package main

import (
    "docker-log-parser/pkg/sqlexplain"
    "fmt"
)

func main() {
    // Create queries with explain plans
    set1 := []sqlexplain.QueryWithPlan{
        {
            Query:           "SELECT * FROM users WHERE email = $1",
            NormalizedQuery: "SELECT * FROM users WHERE email = $N",
            OperationName:   "GetUser",
            Timestamp:       1000,
            DurationMS:      125.5,
            TableName:       "users",
            ExplainPlan:     `[{"Plan": {...}}]`, // PostgreSQL EXPLAIN JSON
        },
    }

    set2 := []sqlexplain.QueryWithPlan{
        {
            Query:           "SELECT * FROM users WHERE email = $1",
            NormalizedQuery: "SELECT * FROM users WHERE email = $N",
            OperationName:   "GetUser",
            Timestamp:       1000,
            DurationMS:      25.5,
            TableName:       "users",
            ExplainPlan:     `[{"Plan": {...}}]`, // Different plan
        },
    }

    // Compare the sets
    comparison := sqlexplain.CompareQuerySets(set1, set2)

    // Print summary
    fmt.Printf("Total queries: %d vs %d\n", 
        comparison.Summary.TotalQueriesSet1,
        comparison.Summary.TotalQueriesSet2)
    fmt.Printf("Common queries: %d\n", comparison.Summary.CommonQueries)
    fmt.Printf("Queries with plan changes: %d\n", 
        comparison.Summary.QueriesWithPlanChange)

    // Check for performance improvements
    for _, perf := range comparison.PerformanceDifferences {
        fmt.Printf("Query: %s\n", perf.ExampleQuery)
        fmt.Printf("Duration change: %.1f%%\n", perf.DurationDiffPct)
        if len(perf.PlanDifferences) > 0 {
            fmt.Printf("Plan changes:\n")
            for _, diff := range perf.PlanDifferences {
                fmt.Printf("  - %s\n", diff)
            }
        }
    }
}
```

### Analyzing Index Usage

```go
package main

import (
    "docker-log-parser/pkg/sqlexplain"
    "fmt"
)

func main() {
    queries := []sqlexplain.QueryWithPlan{
        {
            Query:           "SELECT * FROM users WHERE email = $1",
            NormalizedQuery: "SELECT * FROM users WHERE email = $N",
            DurationMS:      125.5,
            TableName:       "users",
            ExplainPlan:     `[{"Plan": {"Node Type": "Seq Scan", ...}}]`,
        },
    }

    // Analyze index usage
    analysis := sqlexplain.AnalyzeIndexUsage(queries)

    // Print summary
    fmt.Printf("Total queries analyzed: %d\n", analysis.Summary.TotalQueries)
    fmt.Printf("Sequential scans detected: %d\n", 
        analysis.Summary.SequentialScans)
    fmt.Printf("Index scans: %d\n", analysis.Summary.IndexScans)
    fmt.Printf("Recommendations: %d (%d high priority)\n",
        analysis.Summary.TotalRecommendations,
        analysis.Summary.HighPriorityRecs)

    // Print recommendations
    for i, rec := range analysis.Recommendations {
        fmt.Printf("\n%d. [%s] %s\n", i+1, rec.Priority, rec.TableName)
        fmt.Printf("   Columns: %v\n", rec.Columns)
        fmt.Printf("   Reason: %s\n", rec.Reason)
        fmt.Printf("   Impact: %s\n", rec.EstimatedImpact)
        fmt.Printf("   SQL: %s\n", rec.SQLCommand)
    }
}
```

### Using with cmd/compare Tool

The `cmd/compare` package includes helper functions to make it easy to use these analyzers:

```go
package main

import (
    "docker-log-parser/cmd/compare"
    "fmt"
)

func main() {
    // Your existing SQLQuery slices from compare tool
    queries1 := []compare.SQLQuery{...}
    queries2 := []compare.SQLQuery{...}

    // Compare query sets with explain plan analysis
    comparison := compare.CompareQuerySetsWithExplainPlans(
        queries1, queries2, "Run1", "Run2")

    // Analyze index usage
    indexAnalysis := compare.AnalyzeIndexUsageForQueries(
        queries1, "Run1")

    // Format recommendations as readable text
    recommendations := compare.FormatIndexRecommendations(indexAnalysis)
    fmt.Println(recommendations)
}
```

## How It Works

### Query Sorting

Queries are sorted by:
1. **OperationName** (e.g., GraphQL operation) - groups related queries
2. **Timestamp** - maintains execution order within each operation

This allows for meaningful comparison even when queries execute in different orders.

### Explain Plan Parsing

The analyzers parse PostgreSQL EXPLAIN (ANALYZE, FORMAT JSON) output:

```json
[
  {
    "Plan": {
      "Node Type": "Seq Scan",
      "Relation Name": "users",
      "Startup Cost": 0.00,
      "Total Cost": 15234.00,
      "Plan Rows": 10000,
      "Actual Rows": 9500
    }
  }
]
```

Key metrics extracted:
- **Node Type**: Seq Scan, Index Scan, Index Only Scan, etc.
- **Costs**: Startup and total cost estimates
- **Rows**: Estimated vs actual row counts
- **Index Name**: Which index is being used (if any)

### Index Recommendation Priority

Recommendations are prioritized based on:

1. **Frequency** (occurrences)
2. **Query cost** (from EXPLAIN)
3. **Table size** (estimated rows)
4. **Query duration** (actual execution time)

**Priority Levels:**
- **High**: Score ≥ 6 (frequent, expensive, large table)
- **Medium**: Score ≥ 3 (moderate impact)
- **Low**: Score < 3 (minor impact)

### Column Detection

The analyzer attempts to extract column names from filter conditions:

```
Filter: "(email = 'test@example.com'::text)"  → Column: "email"
Filter: "(user_id = 123 AND status = 'active')" → Columns: ["user_id", "status"]
```

For complex filters where columns can't be determined, generic recommendations are provided.

## Performance Considerations

- **Memory**: Parses and stores plan data in memory
- **Computation**: O(n) for single set analysis, O(n*m) for comparison
- **JSON Parsing**: Uses standard library `encoding/json`

For large query sets (>10,000 queries), consider:
- Processing in batches
- Filtering by operation name first
- Focusing on slow queries only

## Testing

Comprehensive tests are included:

```bash
go test ./pkg/sqlexplain -v
go test ./cmd/compare -v -run TestConvert
go test ./cmd/compare -v -run TestAnalyze
```

## Integration

### With Docker Log Viewer

The analyzers integrate with the Docker Log Viewer's SQL query tracking:

1. Queries are extracted from logs (`pkg/logs`)
2. Stored with execution metadata (`pkg/store`)
3. EXPLAIN plans can be fetched via `/api/explain`
4. Analyzers provide insights for optimization

### With Compare Tool

The `cmd/compare` tool can use these analyzers to:

- Compare API endpoint performance
- Identify query differences between environments
- Track performance regressions
- Generate optimization reports

## Future Enhancements

Potential improvements:

- Support for other database types (MySQL, etc.)
- More sophisticated column detection
- Index bloat detection
- Duplicate index identification
- Query plan visualization
- Historical trend analysis

## References

- [PostgreSQL EXPLAIN Documentation](https://www.postgresql.org/docs/current/sql-explain.html)
- [PostgreSQL Index Types](https://www.postgresql.org/docs/current/indexes-types.html)
- [Understanding EXPLAIN Plans](https://www.postgresql.org/docs/current/using-explain.html)
