package sqlexplain

import (
	"encoding/json"
	"testing"
)

func TestAnalyzeIndexUsage(t *testing.T) {
	// Create sample queries with explain plans
	seqScanPlan := `[{"Plan": {
		"Node Type": "Seq Scan",
		"Relation Name": "users",
		"Startup Cost": 0.00,
		"Total Cost": 15234.00,
		"Plan Rows": 10000,
		"Plan Width": 100,
		"Actual Rows": 9500,
		"Actual Loops": 1,
		"Filter": "(email = 'test@example.com'::text)"
	}}]`

	indexScanPlan := `[{"Plan": {
		"Node Type": "Index Scan",
		"Relation Name": "posts",
		"Index Name": "idx_posts_user_id",
		"Startup Cost": 0.29,
		"Total Cost": 45.32,
		"Plan Rows": 100,
		"Plan Width": 50,
		"Actual Rows": 95,
		"Actual Loops": 1
	}}]`

	queries := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			DurationMS:      125.5,
			QueriedTable:    "users",
			ExplainPlan:     seqScanPlan,
		},
		{
			Query:           "SELECT * FROM users WHERE email = $1",
			NormalizedQuery: "SELECT * FROM users WHERE email = $N",
			DurationMS:      130.0,
			QueriedTable:    "users",
			ExplainPlan:     seqScanPlan,
		},
		{
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			DurationMS:      15.0,
			QueriedTable:    "posts",
			ExplainPlan:     indexScanPlan,
		},
	}

	analysis := AnalyzeIndexUsage(queries)

	// Verify summary
	if analysis.Summary.TotalQueries != 3 {
		t.Errorf("Expected TotalQueries = 3, got %d", analysis.Summary.TotalQueries)
	}
	if analysis.Summary.QueriesWithPlans != 3 {
		t.Errorf("Expected QueriesWithPlans = 3, got %d", analysis.Summary.QueriesWithPlans)
	}
	if analysis.Summary.SequentialScans != 2 {
		t.Errorf("Expected SequentialScans = 2, got %d", analysis.Summary.SequentialScans)
	}
	if analysis.Summary.IndexScans != 1 {
		t.Errorf("Expected IndexScans = 1, got %d", analysis.Summary.IndexScans)
	}

	// Verify sequential scan issues
	if len(analysis.SequentialScans) != 1 {
		t.Fatalf("Expected 1 sequential scan issue, got %d", len(analysis.SequentialScans))
	}
	seqScan := analysis.SequentialScans[0]
	if seqScan.QueriedTable != "users" {
		t.Errorf("Expected QueriedTable = 'users', got '%s'", seqScan.QueriedTable)
	}
	if seqScan.Occurrences != 2 {
		t.Errorf("Expected Occurrences = 2, got %d", seqScan.Occurrences)
	}
	if seqScan.EstimatedRows != 10000 {
		t.Errorf("Expected EstimatedRows = 10000, got %f", seqScan.EstimatedRows)
	}

	// Verify index usage stats
	if len(analysis.IndexUsageStats) != 1 {
		t.Fatalf("Expected 1 index usage stat, got %d", len(analysis.IndexUsageStats))
	}
	indexStat := analysis.IndexUsageStats[0]
	if indexStat.IndexName != "idx_posts_user_id" {
		t.Errorf("Expected IndexName = 'idx_posts_user_id', got '%s'", indexStat.IndexName)
	}
	if indexStat.UseCount != 1 {
		t.Errorf("Expected UseCount = 1, got %d", indexStat.UseCount)
	}

	// Verify recommendations
	if analysis.Summary.TotalRecommendations == 0 {
		t.Error("Expected at least one recommendation")
	}
}

func TestAnalyzeIndexUsageEmpty(t *testing.T) {
	analysis := AnalyzeIndexUsage([]QueryWithPlan{})

	if analysis.Summary.TotalQueries != 0 {
		t.Errorf("Expected TotalQueries = 0, got %d", analysis.Summary.TotalQueries)
	}
	if len(analysis.Recommendations) != 0 {
		t.Errorf("Expected 0 recommendations for empty input, got %d", len(analysis.Recommendations))
	}
}

func TestDeterminePriority(t *testing.T) {
	tests := []struct {
		name     string
		issue    SequentialScanIssue
		expected string
	}{
		{
			name: "high priority - frequent, expensive, large table",
			issue: SequentialScanIssue{
				Occurrences:   15,
				Cost:          20000,
				EstimatedRows: 50000,
				DurationMS:    200,
			},
			expected: "high",
		},
		{
			name: "medium priority - moderate frequency and cost",
			issue: SequentialScanIssue{
				Occurrences:   3,
				Cost:          800,
				EstimatedRows: 2000,
				DurationMS:    40,
			},
			expected: "medium",
		},
		{
			name: "low priority - single occurrence, small table",
			issue: SequentialScanIssue{
				Occurrences:   1,
				Cost:          50,
				EstimatedRows: 100,
				DurationMS:    10,
			},
			expected: "low",
		},
		{
			name: "high priority - very frequent",
			issue: SequentialScanIssue{
				Occurrences:   20,
				Cost:          500,
				EstimatedRows: 1000,
				DurationMS:    150,
			},
			expected: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determinePriority(&tt.issue)
			if result != tt.expected {
				t.Errorf("determinePriority() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestEstimateImpact(t *testing.T) {
	tests := []struct {
		name     string
		issue    SequentialScanIssue
		contains string
	}{
		{
			name: "high impact",
			issue: SequentialScanIssue{
				EstimatedRows: 15000,
				Occurrences:   10,
			},
			contains: "High",
		},
		{
			name: "medium impact",
			issue: SequentialScanIssue{
				EstimatedRows: 2000,
				Occurrences:   3,
			},
			contains: "Medium",
		},
		{
			name: "low impact",
			issue: SequentialScanIssue{
				EstimatedRows: 100,
				Occurrences:   1,
			},
			contains: "Low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateImpact(&tt.issue)
			if result[:len(tt.contains)] != tt.contains {
				t.Errorf("estimateImpact() should start with %s, got %s", tt.contains, result)
			}
		})
	}
}

func TestExtractColumnsFromFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		expected []string
	}{
		{
			name:     "simple equality",
			filter:   "(email = 'test@example.com'::text)",
			expected: []string{"email"},
		},
		{
			name:     "multiple conditions",
			filter:   "(status = 'active'::text AND deleted_at IS NULL)",
			expected: []string{"status", "deleted_at"},
		},
		{
			name:     "complex condition",
			filter:   "((user_id)::integer = 123)",
			expected: []string{"user_id"},
		},
		{
			name:     "empty filter",
			filter:   "",
			expected: nil,
		},
		{
			name:     "no clear columns",
			filter:   "(true)",
			expected: []string{},
		},
		{
			name:     "comparison operators",
			filter:   "(age > 18 AND created_at < NOW())",
			expected: []string{"age", "created_at"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractColumnsFromFilter(tt.filter)
			if len(result) != len(tt.expected) {
				t.Errorf("extractColumnsFromFilter() returned %d columns, expected %d", len(result), len(tt.expected))
				t.Logf("Got: %v, Expected: %v", result, tt.expected)
				return
			}
			for i, col := range result {
				if col != tt.expected[i] {
					t.Errorf("Column %d: got %s, expected %s", i, col, tt.expected[i])
				}
			}
		})
	}
}

func TestIsValidColumnName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple", "user_id", true},
		{"valid underscore prefix", "_internal", true},
		{"valid mixed case", "userName", true},
		{"invalid number prefix", "123user", false},
		{"invalid special char", "user-id", false},
		{"invalid empty", "", false},
		{"valid all caps", "USER_ID", true},
		{"valid with numbers", "user123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidColumnName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidColumnName(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "with duplicates",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "no duplicates",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all same",
			input:    []string{"a", "a", "a"},
			expected: []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("deduplicateStrings() returned %d items, expected %d", len(result), len(tt.expected))
				return
			}
			for i, s := range result {
				if s != tt.expected[i] {
					t.Errorf("Item %d: got %s, expected %s", i, s, tt.expected[i])
				}
			}
		})
	}
}

func TestGenerateRecommendations(t *testing.T) {
	seqScans := map[string]*SequentialScanIssue{
		"query1": {
			QueriedTable:    "users",
			EstimatedRows:   10000,
			Cost:            5000,
			Occurrences:     10,
			DurationMS:      150,
			FilterCondition: "(email = $1)",
		},
		"query2": {
			QueriedTable:    "posts",
			EstimatedRows:   100,
			Cost:            50,
			Occurrences:     1,
			DurationMS:      5,
			FilterCondition: "(id = $1)",
		},
	}

	indexUsage := make(map[string]*IndexUsageStat)

	recs := generateRecommendations(seqScans, indexUsage)

	if len(recs) == 0 {
		t.Error("Expected at least one recommendation")
	}

	// High priority should come first
	if len(recs) > 1 {
		for i := 0; i < len(recs)-1; i++ {
			priorities := map[string]int{"high": 0, "medium": 1, "low": 2}
			if priorities[recs[i].Priority] > priorities[recs[i+1].Priority] {
				t.Errorf("Recommendations not sorted by priority: %s before %s",
					recs[i].Priority, recs[i+1].Priority)
			}
		}
	}

	// Verify recommendation fields
	for _, rec := range recs {
		if rec.QueriedTable == "" {
			t.Error("Recommendation missing QueriedTable")
		}
		if rec.Reason == "" {
			t.Error("Recommendation missing Reason")
		}
		if rec.SQLCommand == "" {
			t.Error("Recommendation missing SQLCommand")
		}
		if rec.Priority == "" {
			t.Error("Recommendation missing Priority")
		}
	}
}

func TestSortRecommendations(t *testing.T) {
	recs := []IndexRecommendation{
		{Priority: "low", QueriedTable: "table1"},
		{Priority: "high", QueriedTable: "table2"},
		{Priority: "medium", QueriedTable: "table3"},
		{Priority: "high", QueriedTable: "table4"},
	}

	sortRecommendations(recs)

	// Verify order: high, high, medium, low
	if recs[0].Priority != "high" {
		t.Errorf("First rec should be high priority, got %s", recs[0].Priority)
	}
	if recs[1].Priority != "high" {
		t.Errorf("Second rec should be high priority, got %s", recs[1].Priority)
	}
	if recs[2].Priority != "medium" {
		t.Errorf("Third rec should be medium priority, got %s", recs[2].Priority)
	}
	if recs[3].Priority != "low" {
		t.Errorf("Fourth rec should be low priority, got %s", recs[3].Priority)
	}
}

func TestIndexAnalysisJSON(t *testing.T) {
	// Test that we can marshal and unmarshal the analysis result
	seqScanPlan := `[{"Plan": {
		"Node Type": "Seq Scan",
		"Relation Name": "users",
		"Total Cost": 100.0,
		"Plan Rows": 1000
	}}]`

	queries := []QueryWithPlan{
		{
			Query:           "SELECT * FROM users",
			NormalizedQuery: "SELECT * FROM users",
			ExplainPlan:     seqScanPlan,
		},
	}

	analysis := AnalyzeIndexUsage(queries)

	// Marshal to JSON
	data, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("Failed to marshal analysis: %v", err)
	}

	// Unmarshal back
	var unmarshaled IndexAnalysis
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal analysis: %v", err)
	}

	// Verify basic fields
	if unmarshaled.Summary.TotalQueries != analysis.Summary.TotalQueries {
		t.Error("JSON marshaling/unmarshaling changed TotalQueries")
	}
}

func TestExtractFilterCondition(t *testing.T) {
	plan := &ParsedExplainPlan{
		RawPlan: map[string]any{
			"Filter": "(user_id = 123)",
		},
	}

	result := extractFilterCondition(plan)
	if result != "(user_id = 123)" {
		t.Errorf("Expected filter condition, got %s", result)
	}

	// Test with Index Cond
	plan2 := &ParsedExplainPlan{
		RawPlan: map[string]any{
			"Index Cond": "(email = 'test@example.com')",
		},
	}

	result2 := extractFilterCondition(plan2)
	if result2 != "(email = 'test@example.com')" {
		t.Errorf("Expected index condition, got %s", result2)
	}

	// Test with no filter
	plan3 := &ParsedExplainPlan{
		RawPlan: map[string]any{},
	}

	result3 := extractFilterCondition(plan3)
	if result3 != "" {
		t.Errorf("Expected empty string, got %s", result3)
	}
}
