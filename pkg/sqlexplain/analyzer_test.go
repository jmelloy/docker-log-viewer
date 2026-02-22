package sqlexplain

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCompareQuerySets(t *testing.T) {
	// Create sample explain plans
	plan1JSON := `[{"Plan": {
		"Node Type": "Seq Scan",
		"Relation Name": "users",
		"Startup Cost": 0.00,
		"Total Cost": 150.50,
		"Plan Rows": 1000,
		"Plan Width": 100,
		"Actual Rows": 1000,
		"Actual Loops": 1
	}}]`

	plan2JSON := `[{"Plan": {
		"Node Type": "Index Scan",
		"Relation Name": "users",
		"Index Name": "idx_users_email",
		"Startup Cost": 0.29,
		"Total Cost": 45.32,
		"Plan Rows": 1000,
		"Plan Width": 100,
		"Actual Rows": 1000,
		"Actual Loops": 1
	}}]`

	set1 := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			OperationName:   "GetUser",
			Timestamp:       1000,
			DurationMS:      125.5,
			QueriedTable:    "users",
			Operation:       "select",
			ExplainPlan:     plan1JSON,
		},
		{
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			OperationName:   "GetPosts",
			Timestamp:       2000,
			DurationMS:      50.0,
			QueriedTable:    "posts",
			Operation:       "select",
		},
	}

	set2 := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			OperationName:   "GetUser",
			Timestamp:       1000,
			DurationMS:      25.5,
			QueriedTable:    "users",
			Operation:       "select",
			ExplainPlan:     plan2JSON,
		},
		{
			Query:           "SELECT * FROM comments WHERE post_id = $1",
			NormalizedQuery: "SELECT * FROM comments WHERE post_id = $N",
			OperationName:   "GetComments",
			Timestamp:       3000,
			DurationMS:      30.0,
			QueriedTable:    "comments",
			Operation:       "select",
		},
	}

	result := CompareQuerySets(set1, set2)

	// Verify summary
	if result.Summary.TotalQueriesSet1 != 2 {
		t.Errorf("Expected TotalQueriesSet1 = 2, got %d", result.Summary.TotalQueriesSet1)
	}
	if result.Summary.TotalQueriesSet2 != 2 {
		t.Errorf("Expected TotalQueriesSet2 = 2, got %d", result.Summary.TotalQueriesSet2)
	}
	if result.Summary.CommonQueries != 1 {
		t.Errorf("Expected CommonQueries = 1, got %d", result.Summary.CommonQueries)
	}

	// Verify queries only in set1
	if len(result.QueriesOnlyInSet1) != 1 {
		t.Errorf("Expected 1 query only in set1, got %d", len(result.QueriesOnlyInSet1))
	}
	if len(result.QueriesOnlyInSet1) > 0 && result.QueriesOnlyInSet1[0].QueriedTable != "posts" {
		t.Errorf("Expected posts query in set1, got %s", result.QueriesOnlyInSet1[0].QueriedTable)
	}

	// Verify queries only in set2
	if len(result.QueriesOnlyInSet2) != 1 {
		t.Errorf("Expected 1 query only in set2, got %d", len(result.QueriesOnlyInSet2))
	}
	if len(result.QueriesOnlyInSet2) > 0 && result.QueriesOnlyInSet2[0].QueriedTable != "comments" {
		t.Errorf("Expected comments query in set2, got %s", result.QueriesOnlyInSet2[0].QueriedTable)
	}

	// Verify common query comparison
	if len(result.CommonQueries) != 1 {
		t.Errorf("Expected 1 common query, got %d", len(result.CommonQueries))
	}
	if len(result.CommonQueries) > 0 {
		common := result.CommonQueries[0]
		if !common.HasPlanChange {
			t.Error("Expected HasPlanChange to be true")
		}
		if len(common.PlanDifferences) == 0 {
			t.Error("Expected plan differences to be detected")
		}
		if common.Plan1 == nil || common.Plan2 == nil {
			t.Error("Expected both plans to be parsed")
		}
		if common.Plan1 != nil && common.Plan1.NodeType != "Seq Scan" {
			t.Errorf("Expected plan1 NodeType = 'Seq Scan', got '%s'", common.Plan1.NodeType)
		}
		if common.Plan2 != nil && common.Plan2.NodeType != "Index Scan" {
			t.Errorf("Expected plan2 NodeType = 'Index Scan', got '%s'", common.Plan2.NodeType)
		}
	}

	// Verify performance differences
	if len(result.PerformanceDifferences) == 0 {
		t.Error("Expected performance differences to be detected")
	}
}

func TestParseExplainPlan(t *testing.T) {
	planJSON := `[{
		"Plan": {
			"Node Type": "Seq Scan",
			"Relation Name": "users",
			"Startup Cost": 0.00,
			"Total Cost": 150.50,
			"Plan Rows": 1000,
			"Plan Width": 100,
			"Actual Rows": 950,
			"Actual Loops": 1,
			"Plans": [
				{
					"Node Type": "Index Scan",
					"Index Name": "idx_users_id",
					"Startup Cost": 0.29,
					"Total Cost": 45.32,
					"Plan Rows": 1,
					"Plan Width": 8
				}
			]
		}
	}]`

	plan := parseExplainPlan(planJSON)

	if plan == nil {
		t.Fatal("Expected plan to be parsed, got nil")
	}

	if plan.NodeType != "Seq Scan" {
		t.Errorf("Expected NodeType = 'Seq Scan', got '%s'", plan.NodeType)
	}
	if plan.RelationName != "users" {
		t.Errorf("Expected RelationName = 'users', got '%s'", plan.RelationName)
	}
	if plan.TotalCost != 150.50 {
		t.Errorf("Expected TotalCost = 150.50, got %f", plan.TotalCost)
	}
	if plan.PlanRows != 1000 {
		t.Errorf("Expected PlanRows = 1000, got %f", plan.PlanRows)
	}
	if plan.ActualRows != 950 {
		t.Errorf("Expected ActualRows = 950, got %f", plan.ActualRows)
	}

	// Check child plans
	if len(plan.Plans) != 1 {
		t.Errorf("Expected 1 child plan, got %d", len(plan.Plans))
	}
	if len(plan.Plans) > 0 {
		child := plan.Plans[0]
		if child.NodeType != "Index Scan" {
			t.Errorf("Expected child NodeType = 'Index Scan', got '%s'", child.NodeType)
		}
		if child.IndexName != "idx_users_id" {
			t.Errorf("Expected IndexName = 'idx_users_id', got '%s'", child.IndexName)
		}
	}
}

func TestComparePlans(t *testing.T) {
	plan1 := &ParsedExplainPlan{
		NodeType:     "Seq Scan",
		RelationName: "users",
		TotalCost:    1000.0,
		PlanRows:     10000,
	}

	plan2 := &ParsedExplainPlan{
		NodeType:     "Index Scan",
		RelationName: "users",
		IndexName:    "idx_users_email",
		TotalCost:    50.0,
		PlanRows:     10000,
	}

	diffs := comparePlans(plan1, plan2)

	if len(diffs) == 0 {
		t.Error("Expected differences to be found")
	}

	// Check for node type change
	found := slices.Contains(diffs, "Node type changed: Seq Scan → Index Scan")
	if !found {
		t.Error("Expected node type change to be detected")
	}

	// Check for cost change
	found = slices.Contains(diffs, "Changed from sequential scan to indexed access")
	if !found {
		t.Error("Expected scan type change to be detected")
	}
}

func TestSortQueries(t *testing.T) {
	queries := []QueryWithPlan{
		{OperationName: "B", Timestamp: 2000},
		{OperationName: "A", Timestamp: 3000},
		{OperationName: "A", Timestamp: 1000},
		{OperationName: "C", Timestamp: 1000},
	}

	sortQueries(queries)

	// Verify order: A(1000), A(3000), B(2000), C(1000)
	if queries[0].OperationName != "A" || queries[0].Timestamp != 1000 {
		t.Errorf("Expected first query: A(1000), got %s(%d)", queries[0].OperationName, queries[0].Timestamp)
	}
	if queries[1].OperationName != "A" || queries[1].Timestamp != 3000 {
		t.Errorf("Expected second query: A(3000), got %s(%d)", queries[1].OperationName, queries[1].Timestamp)
	}
	if queries[2].OperationName != "B" {
		t.Errorf("Expected third query: B, got %s", queries[2].OperationName)
	}
	if queries[3].OperationName != "C" {
		t.Errorf("Expected fourth query: C, got %s", queries[3].OperationName)
	}
}

func TestGroupQueriesByNormalized(t *testing.T) {
	queries := []QueryWithPlan{
		{NormalizedQuery: "SELECT * FROM users WHERE id = $N", OperationName: "GetUser"},
		{NormalizedQuery: "SELECT * FROM users WHERE id = $N", OperationName: "GetUser"},
		{NormalizedQuery: "SELECT * FROM posts WHERE id = $N", OperationName: "GetPost"},
	}

	groups := groupQueriesByNormalized(queries)

	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}

	userQueries := groups["SELECT * FROM users WHERE id = $N"]
	if len(userQueries) != 2 {
		t.Errorf("Expected 2 user queries, got %d", len(userQueries))
	}

	postQueries := groups["SELECT * FROM posts WHERE id = $N"]
	if len(postQueries) != 1 {
		t.Errorf("Expected 1 post query, got %d", len(postQueries))
	}
}

func TestCompareQueryGroup(t *testing.T) {
	plan1JSON := `[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 100.0}}]`
	plan2JSON := `[{"Plan": {"Node Type": "Index Scan", "Total Cost": 20.0}}]`

	set1 := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			DurationMS:      50.0,
			ExplainPlan:     plan1JSON,
		},
		{
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			DurationMS:      60.0,
			ExplainPlan:     plan1JSON,
		},
	}

	set2 := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			DurationMS:      10.0,
			ExplainPlan:     plan2JSON,
		},
	}

	comp := compareQueryGroup("SELECT * FROM users WHERE id = $N", set1, set2)

	if comp.Set1Count != 2 {
		t.Errorf("Expected Set1Count = 2, got %d", comp.Set1Count)
	}
	if comp.Set2Count != 1 {
		t.Errorf("Expected Set2Count = 1, got %d", comp.Set2Count)
	}
	if comp.Set1AvgDuration != 55.0 {
		t.Errorf("Expected Set1AvgDuration = 55.0, got %f", comp.Set1AvgDuration)
	}
	if comp.Set2AvgDuration != 10.0 {
		t.Errorf("Expected Set2AvgDuration = 10.0, got %f", comp.Set2AvgDuration)
	}
	if !comp.HasPlanChange {
		t.Error("Expected HasPlanChange to be true")
	}
	if comp.Plan1 == nil || comp.Plan2 == nil {
		t.Error("Expected both plans to be parsed")
	}
}

func TestParseExplainPlanInvalid(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"empty string", ""},
		{"invalid json", "not json"},
		{"empty array", "[]"},
		{"no Plan key", `[{"Other": "data"}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := parseExplainPlan(tt.json)
			if plan != nil {
				t.Errorf("Expected nil plan for %s, got %+v", tt.name, plan)
			}
		})
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{5.0, 5.0},
		{-5.0, 5.0},
		{0.0, 0.0},
		{-123.456, 123.456},
	}

	for _, tt := range tests {
		result := abs(tt.input)
		if result != tt.expected {
			t.Errorf("abs(%f) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

func TestExplainPlanComparisonJSON(t *testing.T) {
	// Test that we can marshal and unmarshal the comparison result
	plan1JSON := `[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 100.0}}]`

	set1 := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users",
			NormalizedQuery: "SELECT * FROM users",
			DurationMS:      50.0,
			ExplainPlan:     plan1JSON,
		},
	}

	result := CompareQuerySets(set1, set1)

	// Marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Unmarshal back
	var unmarshaled ExplainPlanComparison
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify basic fields
	if unmarshaled.Summary.TotalQueriesSet1 != result.Summary.TotalQueriesSet1 {
		t.Error("JSON marshaling/unmarshaling changed TotalQueriesSet1")
	}
}
