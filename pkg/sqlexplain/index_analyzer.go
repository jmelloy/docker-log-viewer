package sqlexplain

import (
	"fmt"
	"strings"
)

// IndexRecommendation represents a suggestion to add or modify an index
type IndexRecommendation struct {
	QueriedTable    string   `json:"tableName"`
	Columns         []string `json:"columns"`
	Reason          string   `json:"reason"`
	EstimatedImpact string   `json:"estimatedImpact"`
	Priority        string   `json:"priority"` // "high", "medium", "low"
	SQLCommand      string   `json:"sqlCommand"`
	AffectedQueries int      `json:"affectedQueries"`
}

// IndexAnalysis provides comprehensive index usage analysis
type IndexAnalysis struct {
	Recommendations []IndexRecommendation `json:"recommendations"`
	SequentialScans []SequentialScanIssue `json:"sequentialScans"`
	UnusedIndexes   []string              `json:"unusedIndexes,omitempty"`
	IndexUsageStats []IndexUsageStat      `json:"indexUsageStats"`
	Summary         IndexAnalysisSummary  `json:"summary"`
}

// SequentialScanIssue identifies queries performing sequential scans
type SequentialScanIssue struct {
	Query           string  `json:"query"`
	NormalizedQuery string  `json:"normalizedQuery"`
	QueriedTable    string  `json:"tableName"`
	EstimatedRows   float64 `json:"estimatedRows"`
	ActualRows      float64 `json:"actualRows"`
	Cost            float64 `json:"cost"`
	DurationMS      float64 `json:"durationMs"`
	Occurrences     int     `json:"occurrences"`
	FilterCondition string  `json:"filterCondition,omitempty"`
}

// IndexUsageStat tracks which indexes are being used
type IndexUsageStat struct {
	IndexName    string  `json:"indexName"`
	QueriedTable string  `json:"tableName"`
	UseCount     int     `json:"useCount"`
	AvgCost      float64 `json:"avgCost"`
	ScanType     string  `json:"scanType"` // "Index Scan", "Index Only Scan", "Bitmap Index Scan"
}

// IndexAnalysisSummary provides high-level statistics
type IndexAnalysisSummary struct {
	TotalQueries         int     `json:"totalQueries"`
	QueriesWithPlans     int     `json:"queriesWithPlans"`
	SequentialScans      int     `json:"sequentialScans"`
	IndexScans           int     `json:"indexScans"`
	TotalRecommendations int     `json:"totalRecommendations"`
	HighPriorityRecs     int     `json:"highPriorityRecs"`
	AvgQueryCost         float64 `json:"avgQueryCost"`
}

// AnalyzeIndexUsage analyzes a set of queries for index usage and recommendations
func AnalyzeIndexUsage(queries []QueryWithPlan) *IndexAnalysis {
	analysis := &IndexAnalysis{
		Summary: IndexAnalysisSummary{
			TotalQueries: len(queries),
		},
	}

	if len(queries) == 0 {
		return analysis
	}

	// Track sequential scans by normalized query
	seqScanMap := make(map[string]*SequentialScanIssue)

	// Track index usage
	indexUsageMap := make(map[string]*IndexUsageStat)

	var totalCost float64
	queriesWithPlans := 0

	for _, q := range queries {
		if q.ExplainPlan == "" {
			continue
		}

		queriesWithPlans++
		plan := parseExplainPlan(q.ExplainPlan)
		if plan == nil {
			continue
		}

		totalCost += plan.TotalCost

		// Analyze the plan for issues and index usage
		analyzeNode(plan, q, seqScanMap, indexUsageMap, &analysis.Summary)
	}

	analysis.Summary.QueriesWithPlans = queriesWithPlans
	if queriesWithPlans > 0 {
		analysis.Summary.AvgQueryCost = totalCost / float64(queriesWithPlans)
	}

	// Convert maps to slices
	for _, issue := range seqScanMap {
		analysis.SequentialScans = append(analysis.SequentialScans, *issue)
	}
	for _, stat := range indexUsageMap {
		analysis.IndexUsageStats = append(analysis.IndexUsageStats, *stat)
	}

	// Generate recommendations based on findings
	analysis.Recommendations = generateRecommendations(seqScanMap, indexUsageMap)
	analysis.Summary.TotalRecommendations = len(analysis.Recommendations)

	for _, rec := range analysis.Recommendations {
		if rec.Priority == "high" {
			analysis.Summary.HighPriorityRecs++
		}
	}

	return analysis
}

// analyzeNode recursively analyzes a plan node
func analyzeNode(plan *ParsedExplainPlan, query QueryWithPlan, seqScanMap map[string]*SequentialScanIssue,
	indexUsageMap map[string]*IndexUsageStat, summary *IndexAnalysisSummary) {

	// Check for sequential scans
	if plan.NodeType == "Seq Scan" {
		summary.SequentialScans++

		key := query.NormalizedQuery
		if issue, exists := seqScanMap[key]; exists {
			issue.Occurrences++
			issue.DurationMS += query.DurationMS
			if plan.TotalCost > issue.Cost {
				issue.Cost = plan.TotalCost
			}
		} else {
			seqScanMap[key] = &SequentialScanIssue{
				Query:           query.Query,
				NormalizedQuery: query.NormalizedQuery,
				QueriedTable:    plan.RelationName,
				EstimatedRows:   plan.PlanRows,
				ActualRows:      plan.ActualRows,
				Cost:            plan.TotalCost,
				DurationMS:      query.DurationMS,
				Occurrences:     1,
				FilterCondition: extractFilterCondition(plan),
			}
		}
	}

	// Track index scans
	if strings.Contains(plan.NodeType, "Index") {
		summary.IndexScans++

		if plan.IndexName != "" {
			key := plan.IndexName
			if stat, exists := indexUsageMap[key]; exists {
				stat.UseCount++
				stat.AvgCost = (stat.AvgCost*float64(stat.UseCount-1) + plan.TotalCost) / float64(stat.UseCount)
			} else {
				indexUsageMap[key] = &IndexUsageStat{
					IndexName:    plan.IndexName,
					QueriedTable: plan.RelationName,
					UseCount:     1,
					AvgCost:      plan.TotalCost,
					ScanType:     plan.NodeType,
				}
			}
		}
	}

	// Recursively analyze child nodes
	for _, childPlan := range plan.Plans {
		analyzeNode(&childPlan, query, seqScanMap, indexUsageMap, summary)
	}
}

// extractFilterCondition extracts filter conditions from the plan
func extractFilterCondition(plan *ParsedExplainPlan) string {
	if plan.RawPlan == nil {
		return ""
	}

	// Try to extract filter information
	if filter, ok := plan.RawPlan["Filter"].(string); ok {
		return filter
	}

	if indexCond, ok := plan.RawPlan["Index Cond"].(string); ok {
		return indexCond
	}

	return ""
}

// generateRecommendations creates index recommendations based on analysis
func generateRecommendations(seqScans map[string]*SequentialScanIssue,
	indexUsage map[string]*IndexUsageStat) []IndexRecommendation {

	var recommendations []IndexRecommendation

	// Recommend indexes for frequent sequential scans
	for _, issue := range seqScans {
		if issue.QueriedTable == "" {
			continue
		}

		// Determine priority based on frequency, cost, and row count
		priority := determinePriority(issue)

		// Skip low priority single-occurrence scans on small tables
		if priority == "low" && issue.Occurrences == 1 && issue.EstimatedRows < 100 {
			continue
		}

		columns := extractColumnsFromFilter(issue.FilterCondition)
		if len(columns) == 0 {
			// Generic recommendation if we can't determine columns
			rec := IndexRecommendation{
				QueriedTable: issue.QueriedTable,
				Columns:      []string{"<filter_column>"},
				Reason: fmt.Sprintf("Sequential scan detected on table '%s' (occurred %d times, avg cost: %.2f)",
					issue.QueriedTable, issue.Occurrences, issue.Cost),
				EstimatedImpact: estimateImpact(issue),
				Priority:        priority,
				SQLCommand: fmt.Sprintf("CREATE INDEX idx_%s_<column> ON %s (<filter_column>);",
					issue.QueriedTable, issue.QueriedTable),
				AffectedQueries: issue.Occurrences,
			}
			recommendations = append(recommendations, rec)
		} else {
			// Specific recommendation with detected columns
			indexName := fmt.Sprintf("idx_%s_%s", issue.QueriedTable, strings.Join(columns, "_"))
			rec := IndexRecommendation{
				QueriedTable: issue.QueriedTable,
				Columns:      columns,
				Reason: fmt.Sprintf("Sequential scan on '%s' filtering by %s (occurred %d times)",
					issue.QueriedTable, strings.Join(columns, ", "), issue.Occurrences),
				EstimatedImpact: estimateImpact(issue),
				Priority:        priority,
				SQLCommand: fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
					indexName, issue.QueriedTable, strings.Join(columns, ", ")),
				AffectedQueries: issue.Occurrences,
			}
			recommendations = append(recommendations, rec)
		}
	}

	// Sort recommendations by priority (high first)
	sortRecommendations(recommendations)

	return recommendations
}

// determinePriority calculates recommendation priority
func determinePriority(issue *SequentialScanIssue) string {
	score := 0

	// Factor 1: Frequency
	if issue.Occurrences >= 10 {
		score += 3
	} else if issue.Occurrences >= 5 {
		score += 2
	} else if issue.Occurrences > 1 {
		score += 1
	}

	// Factor 2: Cost
	if issue.Cost > 10000 {
		score += 3
	} else if issue.Cost > 1000 {
		score += 2
	} else if issue.Cost > 100 {
		score += 1
	}

	// Factor 3: Row count
	if issue.EstimatedRows > 10000 {
		score += 2
	} else if issue.EstimatedRows > 1000 {
		score += 1
	}

	// Factor 4: Duration
	if issue.DurationMS > 100 {
		score += 2
	} else if issue.DurationMS > 50 {
		score += 1
	}

	if score >= 6 {
		return "high"
	} else if score >= 3 {
		return "medium"
	}
	return "low"
}

// estimateImpact estimates the potential performance improvement
func estimateImpact(issue *SequentialScanIssue) string {
	if issue.EstimatedRows > 10000 && issue.Occurrences >= 5 {
		return "High - Could significantly reduce query time"
	} else if issue.EstimatedRows > 1000 || issue.Occurrences >= 3 {
		return "Medium - Should improve query performance"
	}
	return "Low - Minor performance improvement expected"
}

// extractColumnsFromFilter attempts to extract column names from filter conditions
func extractColumnsFromFilter(filter string) []string {
	if filter == "" {
		return nil
	}

	var columns []string
	// Simple pattern matching for common filter patterns
	// Example: "(user_id = $1)" -> ["user_id"]
	// Example: "((status)::text = 'active'::text)" -> ["status"]

	// Remove common patterns
	filter = strings.ReplaceAll(filter, "::text", "")
	filter = strings.ReplaceAll(filter, "::integer", "")
	filter = strings.ReplaceAll(filter, "::bigint", "")

	// Look for patterns like "column_name =" or "column_name IS"
	words := strings.Fields(filter)
	for i, word := range words {
		word = strings.Trim(word, "()[]")
		if i < len(words)-1 {
			next := words[i+1]
			if next == "=" || next == "IS" || next == ">" || next == "<" || next == "!=" || next == "<>" {
				// Check if word looks like a column name (alphanumeric + underscore)
				if isValidColumnName(word) {
					columns = append(columns, word)
				}
			}
		}
	}

	return deduplicateStrings(columns)
}

// isValidColumnName checks if a string looks like a valid column name
func isValidColumnName(s string) bool {
	if s == "" {
		return false
	}
	// Must start with letter or underscore
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_') {
		return false
	}
	// Rest can be alphanumeric or underscore
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// deduplicateStrings removes duplicates from a string slice
func deduplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// sortRecommendations sorts recommendations by priority
func sortRecommendations(recs []IndexRecommendation) {
	priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}

	// Use a simple bubble sort since we're modifying in place
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if priorityOrder[recs[i].Priority] > priorityOrder[recs[j].Priority] {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}
