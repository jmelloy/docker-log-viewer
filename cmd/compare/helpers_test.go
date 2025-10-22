package main

import (
	"docker-log-parser/pkg/sqlexplain"
	"strings"
	"testing"
)

func TestConvertToQueryWithPlan(t *testing.T) {
	queries := []SQLQuery{
		{
			Query:      "SELECT * FROM users WHERE id = $1",
			Normalized: "SELECT * FROM users WHERE id = $N",
			Duration:   25.5,
			Table:      "users",
			Operation:  "select",
			Rows:       1,
		},
		{
			Query:      "SELECT * FROM posts WHERE user_id = $1",
			Normalized: "SELECT * FROM posts WHERE user_id = $N",
			Duration:   15.0,
			Table:      "posts",
			Operation:  "select",
			Rows:       10,
		},
	}

	result := ConvertToQueryWithPlan(queries, "GetUserData")

	if len(result) != 2 {
		t.Errorf("Expected 2 queries, got %d", len(result))
	}

	// Check first query
	if result[0].Query != queries[0].Query {
		t.Errorf("Query mismatch: expected %s, got %s", queries[0].Query, result[0].Query)
	}
	if result[0].NormalizedQuery != queries[0].Normalized {
		t.Errorf("NormalizedQuery mismatch: expected %s, got %s", queries[0].Normalized, result[0].NormalizedQuery)
	}
	if result[0].OperationName != "GetUserData" {
		t.Errorf("OperationName should be 'GetUserData', got %s", result[0].OperationName)
	}
	if result[0].DurationMS != 25.5 {
		t.Errorf("DurationMS should be 25.5, got %f", result[0].DurationMS)
	}
	if result[0].QueriedTable != "users" {
		t.Errorf("QueriedTable should be 'users', got %s", result[0].QueriedTable)
	}

	// Check ordering
	if result[0].Timestamp >= result[1].Timestamp {
		t.Error("Queries should be ordered by timestamp")
	}
}

func TestConvertToQueryWithPlanEmpty(t *testing.T) {
	result := ConvertToQueryWithPlan([]SQLQuery{}, "Operation")
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d queries", len(result))
	}
}

func TestCompareQuerySetsWithExplainPlans(t *testing.T) {
	queries1 := []SQLQuery{
		{
			Query:      "SELECT * FROM users WHERE id = $1",
			Normalized: "SELECT * FROM users WHERE id = $N",
			Duration:   50.0,
			Table:      "users",
		},
	}

	queries2 := []SQLQuery{
		{
			Query:      "SELECT * FROM users WHERE id = $1",
			Normalized: "SELECT * FROM users WHERE id = $N",
			Duration:   25.0,
			Table:      "users",
		},
	}

	result := CompareQuerySetsWithExplainPlans(queries1, queries2, "Op1", "Op2")

	if result == nil {
		t.Fatal("Expected comparison result, got nil")
	}

	if result.Summary.TotalQueriesSet1 != 1 {
		t.Errorf("Expected 1 query in set1, got %d", result.Summary.TotalQueriesSet1)
	}
	if result.Summary.TotalQueriesSet2 != 1 {
		t.Errorf("Expected 1 query in set2, got %d", result.Summary.TotalQueriesSet2)
	}
	if result.Summary.CommonQueries != 1 {
		t.Errorf("Expected 1 common query, got %d", result.Summary.CommonQueries)
	}
}

func TestAnalyzeIndexUsageForQueries(t *testing.T) {
	queries := []SQLQuery{
		{
			Query:      "SELECT * FROM users WHERE email = $1",
			Normalized: "SELECT * FROM users WHERE email = $N",
			Duration:   125.5,
			Table:      "users",
		},
	}

	result := AnalyzeIndexUsageForQueries(queries, "GetUser")

	if result == nil {
		t.Fatal("Expected analysis result, got nil")
	}

	if result.Summary.TotalQueries != 1 {
		t.Errorf("Expected 1 total query, got %d", result.Summary.TotalQueries)
	}
}

func TestFormatIndexRecommendations(t *testing.T) {
	analysis := &sqlexplain.IndexAnalysis{
		Summary: sqlexplain.IndexAnalysisSummary{
			TotalRecommendations: 2,
			HighPriorityRecs:     1,
		},
		Recommendations: []sqlexplain.IndexRecommendation{
			{
				QueriedTable:    "users",
				Columns:         []string{"email"},
				Reason:          "Frequent sequential scan",
				EstimatedImpact: "High - Could significantly reduce query time",
				Priority:        "high",
				SQLCommand:      "CREATE INDEX idx_users_email ON users (email);",
				AffectedQueries: 10,
			},
			{
				QueriedTable:    "posts",
				Columns:         []string{"user_id"},
				Reason:          "Sequential scan detected",
				EstimatedImpact: "Medium - Should improve query performance",
				Priority:        "medium",
				SQLCommand:      "CREATE INDEX idx_posts_user_id ON posts (user_id);",
				AffectedQueries: 3,
			},
		},
	}

	result := FormatIndexRecommendations(analysis)

	if result == "" {
		t.Error("Expected formatted output, got empty string")
	}

	// Check for key information
	if !strings.Contains(result, "2 total") {
		t.Error("Output should mention 2 total recommendations")
	}
	if !strings.Contains(result, "1 high priority") {
		t.Error("Output should mention 1 high priority recommendation")
	}
	if !strings.Contains(result, "users") {
		t.Error("Output should mention users table")
	}
	if !strings.Contains(result, "email") {
		t.Error("Output should mention email column")
	}
	if !strings.Contains(result, "CREATE INDEX") {
		t.Error("Output should include SQL commands")
	}
}

func TestFormatIndexRecommendationsEmpty(t *testing.T) {
	analysis := &sqlexplain.IndexAnalysis{
		Recommendations: []sqlexplain.IndexRecommendation{},
	}

	result := FormatIndexRecommendations(analysis)

	if result != "No index recommendations." {
		t.Errorf("Expected 'No index recommendations.', got %s", result)
	}
}

func TestCompareQuerySetsWithDifferentQueries(t *testing.T) {
	queries1 := []SQLQuery{
		{
			Query:      "SELECT * FROM users",
			Normalized: "SELECT * FROM users",
			Duration:   10.0,
		},
		{
			Query:      "SELECT * FROM posts",
			Normalized: "SELECT * FROM posts",
			Duration:   20.0,
		},
	}

	queries2 := []SQLQuery{
		{
			Query:      "SELECT * FROM users",
			Normalized: "SELECT * FROM users",
			Duration:   5.0,
		},
		{
			Query:      "SELECT * FROM comments",
			Normalized: "SELECT * FROM comments",
			Duration:   15.0,
		},
	}

	result := CompareQuerySetsWithExplainPlans(queries1, queries2, "Run1", "Run2")

	if len(result.QueriesOnlyInSet1) != 1 {
		t.Errorf("Expected 1 query only in set1, got %d", len(result.QueriesOnlyInSet1))
	}
	if len(result.QueriesOnlyInSet2) != 1 {
		t.Errorf("Expected 1 query only in set2, got %d", len(result.QueriesOnlyInSet2))
	}
	if len(result.CommonQueries) != 1 {
		t.Errorf("Expected 1 common query, got %d", len(result.CommonQueries))
	}
}
