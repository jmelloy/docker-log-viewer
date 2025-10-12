# Explain Plan Analyzer Implementation Summary

This document summarizes the explain plan analyzer and index usage analyzer implementation for the Docker Log Viewer project.

## What Was Built

### 1. Explain Plan Comparison Analyzer (`pkg/sqlexplain/analyzer.go`)

A comprehensive tool that compares two sets of SQL queries with their PostgreSQL EXPLAIN plans.

**Key Functions:**
- `CompareQuerySets(set1, set2 []QueryWithPlan) *ExplainPlanComparison`
  - Compares two query sets
  - Returns detailed comparison including plan differences
  - Sorts by operation name and timestamp

**What It Detects:**
- Queries unique to each set
- Common queries with performance differences
- Execution plan changes (e.g., Seq Scan → Index Scan)
- Cost changes (>20% difference flagged)
- Row estimate differences

**Example Use Case:**
Compare production vs staging environments to identify query differences or performance regressions.

### 2. Index Usage Analyzer (`pkg/sqlexplain/index_analyzer.go`)

Analyzes query execution plans to generate index recommendations.

**Key Functions:**
- `AnalyzeIndexUsage(queries []QueryWithPlan) *IndexAnalysis`
  - Analyzes a set of queries for index usage
  - Returns recommendations, sequential scan issues, and statistics

**What It Detects:**
- Sequential scans on large tables
- Missing indexes
- Index usage statistics
- Potential N+1 query patterns

**Recommendation Priority:**
- **High**: Frequent queries (>10x) on large tables (>10k rows) with high cost (>10k)
- **Medium**: Moderate frequency (5-10x) or moderate cost (1k-10k)
- **Low**: Infrequent or low cost queries

**Example Output:**
```
[HIGH] users - Columns: [email]
Reason: Sequential scan on 'users' filtering by email (occurred 15 times)
Impact: High - Could significantly reduce query time
SQL: CREATE INDEX idx_users_email ON users (email);
```

### 3. Helper Functions (`cmd/compare/main.go`)

Convenience functions to integrate the analyzers with the existing compare tool.

**Functions:**
- `ConvertToQueryWithPlan(queries []SQLQuery, operationName string) []QueryWithPlan`
  - Converts compare tool's SQLQuery to analyzer's QueryWithPlan
  
- `CompareQuerySetsWithExplainPlans(queries1, queries2 []SQLQuery, opName1, opName2 string) *ExplainPlanComparison`
  - Wrapper for comparing query sets
  
- `AnalyzeIndexUsageForQueries(queries []SQLQuery, operationName string) *IndexAnalysis`
  - Wrapper for analyzing index usage
  
- `FormatIndexRecommendations(analysis *IndexAnalysis) string`
  - Formats recommendations as human-readable text

## Architecture

### Data Flow

```
SQL Queries (from logs)
    ↓
QueryWithPlan (normalized)
    ↓
CompareQuerySets / AnalyzeIndexUsage
    ↓
ExplainPlanComparison / IndexAnalysis
    ↓
Recommendations / Insights
```

### Key Design Decisions

1. **Sorting**: Queries sorted by OperationName then Timestamp
   - Groups related queries together
   - Maintains execution order
   - Enables meaningful comparison

2. **Normalization**: Parameters replaced with placeholders
   - `$1, $2, $3` → `$N`
   - `'value'` → `'?'`
   - `123` → `N`
   - Groups identical queries with different parameters

3. **Priority Scoring**: Based on multiple factors
   - Frequency: How often the query runs
   - Cost: Query execution cost from EXPLAIN
   - Rows: Table size (estimated rows)
   - Duration: Actual execution time
   - Score ≥6: High, ≥3: Medium, <3: Low

4. **Column Detection**: Heuristic-based extraction
   - Parses filter conditions for column names
   - Falls back to generic recommendation if detection fails

## Testing

Comprehensive test coverage (30+ tests):

**Analyzer Tests (`analyzer_test.go`):**
- Query set comparison
- Explain plan parsing
- Plan difference detection
- Sorting and grouping
- JSON serialization

**Index Analyzer Tests (`index_analyzer_test.go`):**
- Index usage detection
- Sequential scan detection
- Priority calculation
- Column extraction
- Recommendation generation

**Helper Tests (`cmd/compare/helpers_test.go`):**
- Type conversion
- Wrapper functions
- Formatting output

## Integration Points

### With Docker Log Viewer

The analyzers can be integrated with:

1. **Log Collection** (`pkg/logs`)
   - Extract SQL queries from log messages
   - Collect EXPLAIN plans from `/api/explain` endpoint

2. **Storage** (`pkg/store`)
   - SQLQuery model has `ExplainPlan` field
   - Can store and retrieve plans for analysis

3. **Compare Tool** (`cmd/compare`)
   - Helper functions for easy usage
   - Can compare API endpoint performance

### With External Tools

Can be used with:
- PostgreSQL databases (via EXPLAIN JSON)
- GraphQL operation tracking
- Performance monitoring systems
- CI/CD pipelines for regression detection

## Files Created

```
pkg/sqlexplain/
├── analyzer.go              # Explain plan comparison
├── analyzer_test.go         # Tests for comparison
├── index_analyzer.go        # Index usage analysis
├── index_analyzer_test.go   # Tests for index analysis
├── README.md                # Full documentation
├── QUICKSTART.md            # Quick start guide
└── example/
    └── main.go              # Working example

cmd/compare/
├── main.go                  # Updated with helper functions
└── helpers_test.go          # Tests for helpers
```

## Example Usage

### Simple Comparison

```go
comparison := sqlexplain.CompareQuerySets(set1, set2)
fmt.Printf("Queries with plan changes: %d\n", 
    comparison.Summary.QueriesWithPlanChange)
```

### Index Recommendations

```go
analysis := sqlexplain.AnalyzeIndexUsage(queries)
for _, rec := range analysis.Recommendations {
    if rec.Priority == "high" {
        fmt.Println(rec.SQLCommand)
    }
}
```

### With Compare Tool

```go
comparison := compare.CompareQuerySetsWithExplainPlans(
    queries1, queries2, "Prod", "Staging")
analysis := compare.AnalyzeIndexUsageForQueries(queries1, "Prod")
fmt.Println(compare.FormatIndexRecommendations(analysis))
```

## Performance Characteristics

- **Time Complexity**: O(n) for single analysis, O(n*m) for comparison
- **Space Complexity**: O(n) - stores all queries and plans in memory
- **Parsing**: Uses standard `encoding/json` for EXPLAIN plans
- **Scalability**: Suitable for 100s-1000s of queries

For very large datasets (>10k queries):
- Consider filtering by operation name first
- Process in batches
- Focus on slow queries (>100ms)

## Future Enhancements

Possible improvements:

1. **Multi-database Support**
   - MySQL EXPLAIN format
   - SQLite query plans
   
2. **Advanced Analysis**
   - Duplicate index detection
   - Index bloat analysis
   - Query plan visualization
   
3. **Machine Learning**
   - Pattern recognition
   - Predictive recommendations
   - Historical trend analysis

4. **Integration**
   - Real-time monitoring
   - Automated alerts
   - Integration with APM tools

## Metrics

### Code Statistics
- Total lines: ~2,500
- Test coverage: 30+ tests
- Functions: 25+
- Types: 15+

### Documentation
- README.md: 10,000+ characters
- QUICKSTART.md: 5,600+ characters
- Inline comments: Comprehensive
- Example code: Working demo

## Conclusion

The explain plan analyzer and index usage analyzer provide powerful tools for:
- Comparing query performance between environments
- Identifying missing indexes
- Detecting performance regressions
- Optimizing database access patterns

The implementation is:
- Well-tested with comprehensive test coverage
- Well-documented with examples and guides
- Well-integrated with existing codebase
- Production-ready and extensible
