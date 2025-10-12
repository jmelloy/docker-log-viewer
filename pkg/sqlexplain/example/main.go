package main

import (
	"docker-log-parser/pkg/sqlexplain"
	"fmt"
	"strings"
)

func main() {
	fmt.Println("=== Explain Plan Analyzer Example ===\n")

	// Example 1: Compare two query sets
	fmt.Println("1. Comparing Query Sets:")
	fmt.Println(strings.Repeat("-", 50))

	// Set 1: Before optimization (using sequential scan)
	set1 := []sqlexplain.QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			OperationName:   "GetUser",
			Timestamp:       1000,
			DurationMS:      125.5,
			TableName:       "users",
			ExplainPlan: `[{
				"Plan": {
					"Node Type": "Seq Scan",
					"Relation Name": "users",
					"Startup Cost": 0.00,
					"Total Cost": 15234.00,
					"Plan Rows": 10000,
					"Plan Width": 100,
					"Actual Rows": 9500,
					"Actual Loops": 1,
					"Filter": "(email = 'test@example.com'::text)"
				}
			}]`,
		},
		{
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			OperationName:   "GetPosts",
			Timestamp:       2000,
			DurationMS:      50.0,
			TableName:       "posts",
		},
	}

	// Set 2: After optimization (using index scan)
	set2 := []sqlexplain.QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			OperationName:   "GetUser",
			Timestamp:       1000,
			DurationMS:      25.5,
			TableName:       "users",
			ExplainPlan: `[{
				"Plan": {
					"Node Type": "Index Scan",
					"Relation Name": "users",
					"Index Name": "idx_users_email",
					"Startup Cost": 0.29,
					"Total Cost": 45.32,
					"Plan Rows": 10000,
					"Plan Width": 100,
					"Actual Rows": 9500,
					"Actual Loops": 1
				}
			}]`,
		},
		{
			Query:           "SELECT * FROM comments WHERE post_id = $1",
			NormalizedQuery: "SELECT * FROM comments WHERE post_id = $N",
			OperationName:   "GetComments",
			Timestamp:       3000,
			DurationMS:      30.0,
			TableName:       "comments",
		},
	}

	comparison := sqlexplain.CompareQuerySets(set1, set2)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Set 1: %d queries (avg duration: %.2fms)\n",
		comparison.Summary.TotalQueriesSet1,
		comparison.Summary.AvgDurationSet1)
	fmt.Printf("  Set 2: %d queries (avg duration: %.2fms)\n",
		comparison.Summary.TotalQueriesSet2,
		comparison.Summary.AvgDurationSet2)
	fmt.Printf("  Common queries: %d\n", comparison.Summary.CommonQueries)
	fmt.Printf("  Queries with plan changes: %d\n",
		comparison.Summary.QueriesWithPlanChange)

	if len(comparison.CommonQueries) > 0 {
		fmt.Printf("\nCommon Query Analysis:\n")
		for _, cq := range comparison.CommonQueries {
			fmt.Printf("  Query: %s\n", cq.NormalizedQuery)
			fmt.Printf("  Duration change: %.1f%% (%.2fms → %.2fms)\n",
				cq.DurationDiffPct, cq.Set1AvgDuration, cq.Set2AvgDuration)
			if len(cq.PlanDifferences) > 0 {
				fmt.Printf("  Plan changes:\n")
				for _, diff := range cq.PlanDifferences {
					fmt.Printf("    - %s\n", diff)
				}
			}
		}
	}

	if len(comparison.QueriesOnlyInSet1) > 0 {
		fmt.Printf("\nQueries only in Set 1: %d\n", len(comparison.QueriesOnlyInSet1))
		for _, q := range comparison.QueriesOnlyInSet1 {
			fmt.Printf("  - %s (table: %s)\n", q.NormalizedQuery, q.TableName)
		}
	}

	if len(comparison.QueriesOnlyInSet2) > 0 {
		fmt.Printf("\nQueries only in Set 2: %d\n", len(comparison.QueriesOnlyInSet2))
		for _, q := range comparison.QueriesOnlyInSet2 {
			fmt.Printf("  - %s (table: %s)\n", q.NormalizedQuery, q.TableName)
		}
	}

	// Example 2: Analyze index usage
	fmt.Println("\n\n2. Index Usage Analysis:")
	fmt.Println(strings.Repeat("-", 50))

	analysis := sqlexplain.AnalyzeIndexUsage(set1)

	fmt.Printf("Summary:\n")
	fmt.Printf("  Total queries: %d\n", analysis.Summary.TotalQueries)
	fmt.Printf("  Sequential scans: %d\n", analysis.Summary.SequentialScans)
	fmt.Printf("  Index scans: %d\n", analysis.Summary.IndexScans)
	fmt.Printf("  Recommendations: %d (%d high priority)\n",
		analysis.Summary.TotalRecommendations,
		analysis.Summary.HighPriorityRecs)

	if len(analysis.SequentialScans) > 0 {
		fmt.Printf("\nSequential Scan Issues:\n")
		for _, issue := range analysis.SequentialScans {
			fmt.Printf("  Table: %s\n", issue.TableName)
			fmt.Printf("    Occurrences: %d\n", issue.Occurrences)
			fmt.Printf("    Cost: %.2f\n", issue.Cost)
			fmt.Printf("    Estimated rows: %.0f\n", issue.EstimatedRows)
			fmt.Printf("    Duration: %.2fms\n", issue.DurationMS)
			if issue.FilterCondition != "" {
				fmt.Printf("    Filter: %s\n", issue.FilterCondition)
			}
		}
	}

	if len(analysis.Recommendations) > 0 {
		fmt.Printf("\nIndex Recommendations:\n")
		for i, rec := range analysis.Recommendations {
			fmt.Printf("\n%d. [%s] %s\n", i+1, rec.Priority, rec.TableName)
			fmt.Printf("   Columns: %v\n", rec.Columns)
			fmt.Printf("   Reason: %s\n", rec.Reason)
			fmt.Printf("   Impact: %s\n", rec.EstimatedImpact)
			fmt.Printf("   SQL: %s\n", rec.SQLCommand)
			fmt.Printf("   Affected queries: %d\n", rec.AffectedQueries)
		}
	}

	fmt.Println("\n=== Example Complete ===")
}
