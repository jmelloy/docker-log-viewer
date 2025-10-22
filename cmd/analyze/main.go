package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"
)

type Config struct {
	DBPath       string
	ExecutionID1 int64
	ExecutionID2 int64
	OutputFile   string
	Verbose      bool
}

func main() {
	config := parseFlags()

	if config.ExecutionID1 == 0 || config.ExecutionID2 == 0 {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\nError: Both execution IDs are required\n")
		os.Exit(1)
	}

	// Open database
	db, err := store.NewStore(config.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run analysis
	if err := runAnalysis(db, config); err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.DBPath, "db", "graphql-requests.db", "Path to SQLite database")
	flag.Int64Var(&config.ExecutionID1, "exec1", 0, "First execution ID (required)")
	flag.Int64Var(&config.ExecutionID2, "exec2", 0, "Second execution ID (required)")
	flag.StringVar(&config.OutputFile, "output", "", "Output file (optional, defaults to stdout)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output including all queries")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -exec1 <id1> -exec2 <id2> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Analyze and compare SQL queries from two request executions.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	return config
}

func runAnalysis(db *store.Store, config Config) error {
	// Get execution details for both requests
	exec1, err := db.GetExecutionDetail(config.ExecutionID1)
	if err != nil {
		return fmt.Errorf("failed to get execution %d: %w", config.ExecutionID1, err)
	}
	if exec1 == nil {
		return fmt.Errorf("execution %d not found", config.ExecutionID1)
	}

	exec2, err := db.GetExecutionDetail(config.ExecutionID2)
	if err != nil {
		return fmt.Errorf("failed to get execution %d: %w", config.ExecutionID2, err)
	}
	if exec2 == nil {
		return fmt.Errorf("execution %d not found", config.ExecutionID2)
	}

	// Convert to QueryWithPlan format for sqlexplain package
	queries1 := convertToQueryWithPlan(exec1.SQLQueries, fmt.Sprintf("Execution %d", config.ExecutionID1))
	queries2 := convertToQueryWithPlan(exec2.SQLQueries, fmt.Sprintf("Execution %d", config.ExecutionID2))

	// Perform query comparison
	comparison := sqlexplain.CompareQuerySets(queries1, queries2)

	// Analyze index usage for each set
	indexAnalysis1 := sqlexplain.AnalyzeIndexUsage(queries1)
	indexAnalysis2 := sqlexplain.AnalyzeIndexUsage(queries2)

	// Generate output
	output := generateOutput(exec1, exec2, comparison, indexAnalysis1, indexAnalysis2, config.Verbose)

	// Write to file or stdout
	if config.OutputFile != "" {
		if err := os.WriteFile(config.OutputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		log.Printf("Analysis written to %s", config.OutputFile)
	} else {
		fmt.Print(output)
	}

	return nil
}

func convertToQueryWithPlan(queries []store.SQLQuery, operationName string) []sqlexplain.QueryWithPlan {
	result := make([]sqlexplain.QueryWithPlan, 0, len(queries))

	for _, q := range queries {
		qwp := sqlexplain.QueryWithPlan{
			Query:           q.Query,
			NormalizedQuery: q.NormalizedQuery,
			OperationName:   operationName,
			Timestamp:       q.CreatedAt.Unix(),
			DurationMS:      q.DurationMS,
			QueriedTable:    q.QueriedTable,
			Operation:       q.Operation,
			Rows:            q.Rows,
			ExplainPlan:     q.ExplainPlan,
			Variables:       q.Variables,
		}
		result = append(result, qwp)
	}

	return result
}

func generateOutput(exec1, exec2 *store.ExecutionDetail, comparison *sqlexplain.ExplainPlanComparison,
	indexAnalysis1, indexAnalysis2 *sqlexplain.IndexAnalysis, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("=================================================\n")
	sb.WriteString("       SQL Query Analysis Report\n")
	sb.WriteString("=================================================\n\n")

	// Execution Summary
	sb.WriteString("EXECUTION DETAILS\n")
	sb.WriteString(strings.Repeat("-", 50) + "\n")
	sb.WriteString(fmt.Sprintf("Execution 1 (ID: %d)\n", exec1.Execution.ID))
	sb.WriteString(fmt.Sprintf("  Request ID: %s\n", exec1.Execution.RequestIDHeader))
	sb.WriteString(fmt.Sprintf("  Duration: %dms\n", exec1.Execution.DurationMS))
	sb.WriteString(fmt.Sprintf("  Status: %d\n", exec1.Execution.StatusCode))
	sb.WriteString(fmt.Sprintf("  SQL Queries: %d\n", len(exec1.SQLQueries)))
	if exec1.Request != nil {
		sb.WriteString(fmt.Sprintf("  Request Name: %s\n", exec1.Request.Name))
	}
	if exec1.Execution.Server != nil {
		sb.WriteString(fmt.Sprintf("  Server: %s\n", exec1.Execution.Server.Name))
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("Execution 2 (ID: %d)\n", exec2.Execution.ID))
	sb.WriteString(fmt.Sprintf("  Request ID: %s\n", exec2.Execution.RequestIDHeader))
	sb.WriteString(fmt.Sprintf("  Duration: %dms\n", exec2.Execution.DurationMS))
	sb.WriteString(fmt.Sprintf("  Status: %d\n", exec2.Execution.StatusCode))
	sb.WriteString(fmt.Sprintf("  SQL Queries: %d\n", len(exec2.SQLQueries)))
	if exec2.Request != nil {
		sb.WriteString(fmt.Sprintf("  Request Name: %s\n", exec2.Request.Name))
	}
	if exec2.Execution.Server != nil {
		sb.WriteString(fmt.Sprintf("  Server: %s\n", exec2.Execution.Server.Name))
	}
	sb.WriteString("\n\n")

	// Query Comparison Summary
	sb.WriteString("QUERY COMPARISON SUMMARY\n")
	sb.WriteString(strings.Repeat("-", 50) + "\n")
	sb.WriteString(fmt.Sprintf("Total Queries:\n"))
	sb.WriteString(fmt.Sprintf("  Execution 1: %d queries (avg: %.2fms)\n",
		comparison.Summary.TotalQueriesSet1, comparison.Summary.AvgDurationSet1))
	sb.WriteString(fmt.Sprintf("  Execution 2: %d queries (avg: %.2fms)\n",
		comparison.Summary.TotalQueriesSet2, comparison.Summary.AvgDurationSet2))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Unique Queries:\n"))
	sb.WriteString(fmt.Sprintf("  Execution 1: %d unique queries\n", comparison.Summary.UniqueQueriesSet1))
	sb.WriteString(fmt.Sprintf("  Execution 2: %d unique queries\n", comparison.Summary.UniqueQueriesSet2))
	sb.WriteString(fmt.Sprintf("  Common queries: %d\n", comparison.Summary.CommonQueries))
	sb.WriteString(fmt.Sprintf("  Queries with plan changes: %d\n", comparison.Summary.QueriesWithPlanChange))
	sb.WriteString("\n")

	// Performance differences
	if len(comparison.PerformanceDifferences) > 0 {
		sb.WriteString("\nPERFORMANCE DIFFERENCES\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		for i, diff := range comparison.PerformanceDifferences {
			if i >= 10 { // Show top 10
				break
			}
			change := "slower"
			if diff.DurationDiffPct < 0 {
				change = "faster"
			}
			sb.WriteString(fmt.Sprintf("\n%d. Query: %s\n", i+1, diff.NormalizedQuery))
			sb.WriteString(fmt.Sprintf("   Duration: %.2fms → %.2fms (%.1f%% %s)\n",
				diff.Set1AvgDuration, diff.Set2AvgDuration, abs(diff.DurationDiffPct), change))
			sb.WriteString(fmt.Sprintf("   Count: %d → %d\n", diff.Set1Count, diff.Set2Count))
		}
		sb.WriteString("\n")
	}

	// Queries only in execution 1
	if len(comparison.QueriesOnlyInSet1) > 0 {
		sb.WriteString("\nQUERIES ONLY IN EXECUTION 1\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		for i, q := range comparison.QueriesOnlyInSet1 {
			if i >= 10 { // Show top 10
				break
			}
			sb.WriteString(fmt.Sprintf("%d. %s (table: %s, duration: %.2fms)\n",
				i+1, q.NormalizedQuery, q.QueriedTable, q.DurationMS))
		}
		if len(comparison.QueriesOnlyInSet1) > 10 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(comparison.QueriesOnlyInSet1)-10))
		}
		sb.WriteString("\n")
	}

	// Queries only in execution 2
	if len(comparison.QueriesOnlyInSet2) > 0 {
		sb.WriteString("\nQUERIES ONLY IN EXECUTION 2\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		for i, q := range comparison.QueriesOnlyInSet2 {
			if i >= 10 { // Show top 10
				break
			}
			sb.WriteString(fmt.Sprintf("%d. %s (table: %s, duration: %.2fms)\n",
				i+1, q.NormalizedQuery, q.QueriedTable, q.DurationMS))
		}
		if len(comparison.QueriesOnlyInSet2) > 10 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(comparison.QueriesOnlyInSet2)-10))
		}
		sb.WriteString("\n")
	}

	// Index Analysis for Execution 1
	if len(indexAnalysis1.SequentialScans) > 0 {
		sb.WriteString("\nINDEX ANALYSIS - EXECUTION 1\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		sb.WriteString(fmt.Sprintf("Sequential Scans: %d\n", indexAnalysis1.Summary.SequentialScans))
		sb.WriteString(fmt.Sprintf("Index Scans: %d\n", indexAnalysis1.Summary.IndexScans))
		sb.WriteString(fmt.Sprintf("Recommendations: %d (%d high priority)\n\n",
			indexAnalysis1.Summary.TotalRecommendations, indexAnalysis1.Summary.HighPriorityRecs))

		if len(indexAnalysis1.SequentialScans) > 0 {
			sb.WriteString("Sequential Scan Issues:\n")
			for i, issue := range indexAnalysis1.SequentialScans {
				if i >= 5 { // Show top 5
					break
				}
				sb.WriteString(fmt.Sprintf("  %d. Table: %s\n", i+1, issue.QueriedTable))
				sb.WriteString(fmt.Sprintf("     Occurrences: %d, Cost: %.2f, Duration: %.2fms\n",
					issue.Occurrences, issue.Cost, issue.DurationMS))
				if issue.FilterCondition != "" {
					sb.WriteString(fmt.Sprintf("     Filter: %s\n", issue.FilterCondition))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Index Analysis for Execution 2
	if len(indexAnalysis2.SequentialScans) > 0 {
		sb.WriteString("\nINDEX ANALYSIS - EXECUTION 2\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		sb.WriteString(fmt.Sprintf("Sequential Scans: %d\n", indexAnalysis2.Summary.SequentialScans))
		sb.WriteString(fmt.Sprintf("Index Scans: %d\n", indexAnalysis2.Summary.IndexScans))
		sb.WriteString(fmt.Sprintf("Recommendations: %d (%d high priority)\n\n",
			indexAnalysis2.Summary.TotalRecommendations, indexAnalysis2.Summary.HighPriorityRecs))

		if len(indexAnalysis2.SequentialScans) > 0 {
			sb.WriteString("Sequential Scan Issues:\n")
			for i, issue := range indexAnalysis2.SequentialScans {
				if i >= 5 { // Show top 5
					break
				}
				sb.WriteString(fmt.Sprintf("  %d. Table: %s\n", i+1, issue.QueriedTable))
				sb.WriteString(fmt.Sprintf("     Occurrences: %d, Cost: %.2f, Duration: %.2fms\n",
					issue.Occurrences, issue.Cost, issue.DurationMS))
				if issue.FilterCondition != "" {
					sb.WriteString(fmt.Sprintf("     Filter: %s\n", issue.FilterCondition))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Index Recommendations
	if len(indexAnalysis1.Recommendations) > 0 || len(indexAnalysis2.Recommendations) > 0 {
		sb.WriteString("\nINDEX RECOMMENDATIONS\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")

		if len(indexAnalysis1.Recommendations) > 0 {
			sb.WriteString("For Execution 1:\n")
			for i, rec := range indexAnalysis1.Recommendations {
				if i >= 3 { // Show top 3
					break
				}
				sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, rec.Priority, rec.QueriedTable))
				sb.WriteString(fmt.Sprintf("     Columns: %v\n", rec.Columns))
				sb.WriteString(fmt.Sprintf("     Reason: %s\n", rec.Reason))
				sb.WriteString(fmt.Sprintf("     Impact: %s\n", rec.EstimatedImpact))
				sb.WriteString(fmt.Sprintf("     SQL: %s\n", rec.SQLCommand))
			}
			sb.WriteString("\n")
		}

		if len(indexAnalysis2.Recommendations) > 0 {
			sb.WriteString("For Execution 2:\n")
			for i, rec := range indexAnalysis2.Recommendations {
				if i >= 3 { // Show top 3
					break
				}
				sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, rec.Priority, rec.QueriedTable))
				sb.WriteString(fmt.Sprintf("     Columns: %v\n", rec.Columns))
				sb.WriteString(fmt.Sprintf("     Reason: %s\n", rec.Reason))
				sb.WriteString(fmt.Sprintf("     Impact: %s\n", rec.EstimatedImpact))
				sb.WriteString(fmt.Sprintf("     SQL: %s\n", rec.SQLCommand))
			}
			sb.WriteString("\n")
		}
	}

	// Verbose: Show all queries from both executions
	if verbose {
		sb.WriteString("\nDETAILED QUERY LIST - EXECUTION 1\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		for i, q := range exec1.SQLQueries {
			sb.WriteString(fmt.Sprintf("\n%d. Query:\n", i+1))
			sb.WriteString(fmt.Sprintf("   %s\n", q.Query))
			sb.WriteString(fmt.Sprintf("   Table: %s, Operation: %s\n", q.QueriedTable, q.Operation))
			sb.WriteString(fmt.Sprintf("   Duration: %.2fms, Rows: %d\n", q.DurationMS, q.Rows))
			if q.GraphQLOperation != "" {
				sb.WriteString(fmt.Sprintf("   GraphQL Op: %s\n", q.GraphQLOperation))
			}
		}
		sb.WriteString("\n")

		sb.WriteString("\nDETAILED QUERY LIST - EXECUTION 2\n")
		sb.WriteString(strings.Repeat("-", 50) + "\n")
		for i, q := range exec2.SQLQueries {
			sb.WriteString(fmt.Sprintf("\n%d. Query:\n", i+1))
			sb.WriteString(fmt.Sprintf("   %s\n", q.Query))
			sb.WriteString(fmt.Sprintf("   Table: %s, Operation: %s\n", q.QueriedTable, q.Operation))
			sb.WriteString(fmt.Sprintf("   Duration: %.2fms, Rows: %d\n", q.DurationMS, q.Rows))
			if q.GraphQLOperation != "" {
				sb.WriteString(fmt.Sprintf("   GraphQL Op: %s\n", q.GraphQLOperation))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=================================================\n")
	sb.WriteString("              End of Report\n")
	sb.WriteString("=================================================\n")

	return sb.String()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
