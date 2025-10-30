package sqlexplain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// QueryWithPlan represents a SQL query with its explain plan
type QueryWithPlan struct {
	Query           string
	NormalizedQuery string
	OperationName   string // GraphQL operation or other grouping identifier
	Timestamp       int64  // Unix timestamp for ordering
	DurationMS      float64
	QueriedTable    string
	Operation       string
	Rows            int
	ExplainPlan     string // JSON string of the explain plan
	Variables       string
}

// ExplainPlanComparison represents comparison between two query sets
type ExplainPlanComparison struct {
	QueriesOnlyInSet1      []QueryWithPlan       `json:"queriesOnlyInSet1"`
	QueriesOnlyInSet2      []QueryWithPlan       `json:"queriesOnlyInSet2"`
	CommonQueries          []QueryPlanComparison `json:"commonQueries"`
	PlanDifferences        []QueryPlanComparison `json:"planDifferences"`
	PerformanceDifferences []QueryPlanComparison `json:"performanceDifferences"`
	Summary                ComparisonSummary     `json:"summary"`
}

// QueryPlanComparison compares the same query between two sets
type QueryPlanComparison struct {
	NormalizedQuery string             `json:"normalizedQuery"`
	ExampleQuery    string             `json:"exampleQuery"`
	OperationName   string             `json:"operationName"`
	Set1Count       int                `json:"set1Count"`
	Set2Count       int                `json:"set2Count"`
	Set1AvgDuration float64            `json:"set1AvgDuration"`
	Set2AvgDuration float64            `json:"set2AvgDuration"`
	DurationDiffPct float64            `json:"durationDiffPct"`
	Plan1           *ParsedExplainPlan `json:"plan1,omitempty"`
	Plan2           *ParsedExplainPlan `json:"plan2,omitempty"`
	PlanDifferences []string           `json:"planDifferences,omitempty"`
	HasPlanChange   bool               `json:"hasPlanChange"`
}

// ComparisonSummary provides high-level statistics
type ComparisonSummary struct {
	TotalQueriesSet1      int     `json:"totalQueriesSet1"`
	TotalQueriesSet2      int     `json:"totalQueriesSet2"`
	UniqueQueriesSet1     int     `json:"uniqueQueriesSet1"`
	UniqueQueriesSet2     int     `json:"uniqueQueriesSet2"`
	CommonQueries         int     `json:"commonQueries"`
	QueriesWithPlanChange int     `json:"queriesWithPlanChange"`
	AvgDurationSet1       float64 `json:"avgDurationSet1"`
	AvgDurationSet2       float64 `json:"avgDurationSet2"`
}

// ParsedExplainPlan represents a parsed PostgreSQL EXPLAIN plan
type ParsedExplainPlan struct {
	NodeType      string                 `json:"nodeType"`
	RelationName  string                 `json:"relationName,omitempty"`
	StartupCost   float64                `json:"startupCost"`
	TotalCost     float64                `json:"totalCost"`
	PlanRows      float64                `json:"planRows"`
	PlanWidth     int                    `json:"planWidth"`
	ActualRows    float64                `json:"actualRows,omitempty"`
	ActualLoops   int                    `json:"actualLoops,omitempty"`
	IndexName     string                 `json:"indexName,omitempty"`
	ScanDirection string                 `json:"scanDirection,omitempty"`
	Plans         []ParsedExplainPlan    `json:"plans,omitempty"`
	RawPlan       map[string]interface{} `json:"rawPlan,omitempty"`
}

// CompareQuerySets compares two sets of queries with their explain plans
// Queries are sorted by OperationName first, then by Timestamp
func CompareQuerySets(set1, set2 []QueryWithPlan) *ExplainPlanComparison {
	// Sort both sets
	sortQueries(set1)
	sortQueries(set2)

	result := &ExplainPlanComparison{
		Summary: ComparisonSummary{
			TotalQueriesSet1: len(set1),
			TotalQueriesSet2: len(set2),
		},
	}

	// Group queries by normalized form
	map1 := groupQueriesByNormalized(set1)
	map2 := groupQueriesByNormalized(set2)

	result.Summary.UniqueQueriesSet1 = len(map1)
	result.Summary.UniqueQueriesSet2 = len(map2)

	// Calculate average durations
	if len(set1) > 0 {
		var total float64
		for _, q := range set1 {
			total += q.DurationMS
		}
		result.Summary.AvgDurationSet1 = total / float64(len(set1))
	}
	if len(set2) > 0 {
		var total float64
		for _, q := range set2 {
			total += q.DurationMS
		}
		result.Summary.AvgDurationSet2 = total / float64(len(set2))
	}

	// Find queries only in set1
	for norm, queries := range map1 {
		if _, exists := map2[norm]; !exists {
			result.QueriesOnlyInSet1 = append(result.QueriesOnlyInSet1, queries[0])
		}
	}

	// Find queries only in set2
	for norm, queries := range map2 {
		if _, exists := map1[norm]; !exists {
			result.QueriesOnlyInSet2 = append(result.QueriesOnlyInSet2, queries[0])
		}
	}

	// Compare common queries
	for norm, queries1 := range map1 {
		if queries2, exists := map2[norm]; exists {
			comp := compareQueryGroup(norm, queries1, queries2)
			result.CommonQueries = append(result.CommonQueries, comp)

			if comp.HasPlanChange {
				result.PlanDifferences = append(result.PlanDifferences, comp)
				result.Summary.QueriesWithPlanChange++
			}

			// Track significant performance differences (>20% change)
			if comp.DurationDiffPct > 20 || comp.DurationDiffPct < -20 {
				result.PerformanceDifferences = append(result.PerformanceDifferences, comp)
			}
		}
	}

	result.Summary.CommonQueries = len(result.CommonQueries)

	// Sort results by duration difference (most impactful first)
	sort.Slice(result.PerformanceDifferences, func(i, j int) bool {
		return abs(result.PerformanceDifferences[i].DurationDiffPct) >
			abs(result.PerformanceDifferences[j].DurationDiffPct)
	})

	return result
}

// sortQueries sorts by OperationName first, then Timestamp
func sortQueries(queries []QueryWithPlan) {
	sort.Slice(queries, func(i, j int) bool {
		if queries[i].OperationName != queries[j].OperationName {
			return queries[i].OperationName < queries[j].OperationName
		}
		return queries[i].Timestamp < queries[j].Timestamp
	})
}

// groupQueriesByNormalized groups queries by their normalized form
func groupQueriesByNormalized(queries []QueryWithPlan) map[string][]QueryWithPlan {
	result := make(map[string][]QueryWithPlan)
	for _, q := range queries {
		result[q.NormalizedQuery] = append(result[q.NormalizedQuery], q)
	}
	return result
}

// compareQueryGroup compares two groups of the same query
func compareQueryGroup(normalized string, set1, set2 []QueryWithPlan) QueryPlanComparison {
	comp := QueryPlanComparison{
		NormalizedQuery: normalized,
		ExampleQuery:    set1[0].Query,
		OperationName:   set1[0].OperationName,
		Set1Count:       len(set1),
		Set2Count:       len(set2),
	}

	// Calculate average durations
	var total1, total2 float64
	for _, q := range set1 {
		total1 += q.DurationMS
	}
	for _, q := range set2 {
		total2 += q.DurationMS
	}
	comp.Set1AvgDuration = total1 / float64(len(set1))
	comp.Set2AvgDuration = total2 / float64(len(set2))

	// Calculate percentage difference
	if comp.Set1AvgDuration > 0 {
		comp.DurationDiffPct = ((comp.Set2AvgDuration - comp.Set1AvgDuration) / comp.Set1AvgDuration) * 100
	}

	// Parse and compare explain plans if available
	if set1[0].ExplainPlan != "" && set2[0].ExplainPlan != "" {
		plan1 := parseExplainPlan(set1[0].ExplainPlan)
		plan2 := parseExplainPlan(set2[0].ExplainPlan)

		comp.Plan1 = plan1
		comp.Plan2 = plan2

		if plan1 != nil && plan2 != nil {
			comp.PlanDifferences = comparePlans(plan1, plan2)
			comp.HasPlanChange = len(comp.PlanDifferences) > 0
		}
	}

	return comp
}

// parseExplainPlan parses a JSON explain plan string
func parseExplainPlan(planJSON string) *ParsedExplainPlan {
	if planJSON == "" {
		return nil
	}

	var rawPlans []map[string]interface{}
	if err := json.Unmarshal([]byte(planJSON), &rawPlans); err != nil {
		return nil
	}

	if len(rawPlans) == 0 {
		return nil
	}

	// Get the Plan node from the first element
	planData, ok := rawPlans[0]["Plan"].(map[string]interface{})
	if !ok {
		return nil
	}

	return parsePlanNode(planData)
}

// parsePlanNode recursively parses a plan node
func parsePlanNode(data map[string]interface{}) *ParsedExplainPlan {
	plan := &ParsedExplainPlan{
		RawPlan: data,
	}

	// Extract common fields
	if nodeType, ok := data["Node Type"].(string); ok {
		plan.NodeType = nodeType
	}
	if relationName, ok := data["Relation Name"].(string); ok {
		plan.RelationName = relationName
	}
	if startupCost, ok := data["Startup Cost"].(float64); ok {
		plan.StartupCost = startupCost
	}
	if totalCost, ok := data["Total Cost"].(float64); ok {
		plan.TotalCost = totalCost
	}
	if planRows, ok := data["Plan Rows"].(float64); ok {
		plan.PlanRows = planRows
	}
	if planWidth, ok := data["Plan Width"].(float64); ok {
		plan.PlanWidth = int(planWidth)
	}
	if actualRows, ok := data["Actual Rows"].(float64); ok {
		plan.ActualRows = actualRows
	}
	if actualLoops, ok := data["Actual Loops"].(float64); ok {
		plan.ActualLoops = int(actualLoops)
	}
	if indexName, ok := data["Index Name"].(string); ok {
		plan.IndexName = indexName
	}
	if scanDir, ok := data["Scan Direction"].(string); ok {
		plan.ScanDirection = scanDir
	}

	// Parse child plans recursively
	if plans, ok := data["Plans"].([]interface{}); ok {
		for _, p := range plans {
			if planMap, ok := p.(map[string]interface{}); ok {
				plan.Plans = append(plan.Plans, *parsePlanNode(planMap))
			}
		}
	}

	return plan
}

// comparePlans identifies differences between two explain plans
func comparePlans(plan1, plan2 *ParsedExplainPlan) []string {
	var diffs []string

	// Compare node types
	if plan1.NodeType != plan2.NodeType {
		diffs = append(diffs, fmt.Sprintf("Node type changed: %s → %s", plan1.NodeType, plan2.NodeType))
	}

	// Compare index usage
	if plan1.IndexName != plan2.IndexName {
		if plan1.IndexName == "" {
			diffs = append(diffs, fmt.Sprintf("Now using index: %s", plan2.IndexName))
		} else if plan2.IndexName == "" {
			diffs = append(diffs, fmt.Sprintf("No longer using index: %s", plan1.IndexName))
		} else {
			diffs = append(diffs, fmt.Sprintf("Index changed: %s → %s", plan1.IndexName, plan2.IndexName))
		}
	}

	// Compare costs (significant change = >20%)
	costDiff := ((plan2.TotalCost - plan1.TotalCost) / plan1.TotalCost) * 100
	if abs(costDiff) > 20 {
		diffs = append(diffs, fmt.Sprintf("Cost changed by %.1f%% (%.2f → %.2f)",
			costDiff, plan1.TotalCost, plan2.TotalCost))
	}

	// Compare row estimates
	if plan1.PlanRows != plan2.PlanRows {
		rowDiff := ((plan2.PlanRows - plan1.PlanRows) / plan1.PlanRows) * 100
		if abs(rowDiff) > 20 {
			diffs = append(diffs, fmt.Sprintf("Estimated rows changed by %.1f%% (%.0f → %.0f)",
				rowDiff, plan1.PlanRows, plan2.PlanRows))
		}
	}

	// Check for scan type changes
	if strings.Contains(plan1.NodeType, "Seq Scan") && !strings.Contains(plan2.NodeType, "Seq Scan") {
		diffs = append(diffs, "Changed from sequential scan to indexed access")
	} else if !strings.Contains(plan1.NodeType, "Seq Scan") && strings.Contains(plan2.NodeType, "Seq Scan") {
		diffs = append(diffs, "Changed from indexed access to sequential scan")
	}

	return diffs
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
