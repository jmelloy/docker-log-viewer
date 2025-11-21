package main

import (
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"
)

func TestConvertToQueryWithPlan(t *testing.T) {
	now := time.Now()
	queries := []store.SQLQuery{
		{
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			DurationMS:      10.5,
			QueriedTable:    "users",
			Operation:       "SELECT",
			Rows:            1,
			CreatedAt:       now,
		},
		{
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			DurationMS:      25.3,
			QueriedTable:    "posts",
			Operation:       "SELECT",
			Rows:            10,
			CreatedAt:       now,
		},
	}

	result := convertToQueryWithPlan(queries, "TestOp")

	if len(result) != 2 {
		t.Errorf("Expected 2 queries, got %d", len(result))
	}

	if result[0].Query != queries[0].Query {
		t.Errorf("Query mismatch: expected %s, got %s", queries[0].Query, result[0].Query)
	}

	if result[0].OperationName != "TestOp" {
		t.Errorf("OperationName mismatch: expected TestOp, got %s", result[0].OperationName)
	}

	if result[0].DurationMS != 10.5 {
		t.Errorf("DurationMS mismatch: expected 10.5, got %f", result[0].DurationMS)
	}
}

func TestGenerateOutput(t *testing.T) {
	now := time.Now()

	exec1 := &store.RequestDetailResponse{
		Execution: store.Request{
			ID:         1,
			DurationMS: 100,
			StatusCode: 200,
		},
		SQLQueries: []store.SQLQuery{
			{
				Query:           "SELECT * FROM users WHERE id = $1",
				NormalizedQuery: "SELECT * FROM users WHERE id = $N",
				DurationMS:      10.0,
				QueriedTable:    "users",
				Operation:       "SELECT",
				Rows:            1,
				CreatedAt:       now,
			},
		},
	}

	exec2 := &store.RequestDetailResponse{
		Execution: store.Request{
			ID:         2,
			DurationMS: 150,
			StatusCode: 200,
		},
		SQLQueries: []store.SQLQuery{
			{
				Query:           "SELECT * FROM users WHERE id = $1",
				NormalizedQuery: "SELECT * FROM users WHERE id = $N",
				DurationMS:      15.0,
				QueriedTable:    "users",
				Operation:       "SELECT",
				Rows:            1,
				CreatedAt:       now,
			},
		},
	}

	queries1 := convertToQueryWithPlan(exec1.SQLQueries, "Exec1")
	queries2 := convertToQueryWithPlan(exec2.SQLQueries, "Exec2")

	comparison := sqlexplain.CompareQuerySets(queries1, queries2)
	indexAnalysis1 := sqlexplain.AnalyzeIndexUsage(queries1)
	indexAnalysis2 := sqlexplain.AnalyzeIndexUsage(queries2)

	output := generateOutput(exec1, exec2, comparison, indexAnalysis1, indexAnalysis2, false)

	// Verify key sections are present
	if !strings.Contains(output, "SQL Query Analysis Report") {
		t.Error("Output should contain report title")
	}

	if !strings.Contains(output, "EXECUTION DETAILS") {
		t.Error("Output should contain execution details section")
	}

	if !strings.Contains(output, "QUERY COMPARISON SUMMARY") {
		t.Error("Output should contain comparison summary section")
	}

	if !strings.Contains(output, "Execution 1 (ID: 1)") {
		t.Error("Output should contain execution 1 details")
	}

	if !strings.Contains(output, "Execution 2 (ID: 2)") {
		t.Error("Output should contain execution 2 details")
	}

	if !strings.Contains(output, "req-001") {
		t.Error("Output should contain request ID for execution 1")
	}

	if !strings.Contains(output, "req-002") {
		t.Error("Output should contain request ID for execution 2")
	}
}

func TestGenerateOutputVerbose(t *testing.T) {
	now := time.Now()

	exec1 := &store.RequestDetailResponse{
		Execution: store.Request{
			ID:         1,
			DurationMS: 100,
			StatusCode: 200,
		},
		SQLQueries: []store.SQLQuery{
			{
				Query:           "SELECT * FROM users WHERE id = $1",
				NormalizedQuery: "SELECT * FROM users WHERE id = $N",
				DurationMS:      10.0,
				QueriedTable:    "users",
				Operation:       "SELECT",
				Rows:            1,
				CreatedAt:       now,
			},
		},
	}

	exec2 := &store.RequestDetailResponse{
		Execution: store.Request{
			ID:         2,
			DurationMS: 150,
			StatusCode: 200,
		},
		SQLQueries: []store.SQLQuery{},
	}

	queries1 := convertToQueryWithPlan(exec1.SQLQueries, "Exec1")
	queries2 := convertToQueryWithPlan(exec2.SQLQueries, "Exec2")

	comparison := sqlexplain.CompareQuerySets(queries1, queries2)
	indexAnalysis1 := sqlexplain.AnalyzeIndexUsage(queries1)
	indexAnalysis2 := sqlexplain.AnalyzeIndexUsage(queries2)

	output := generateOutput(exec1, exec2, comparison, indexAnalysis1, indexAnalysis2, true)

	// Verbose mode should include detailed query lists
	if !strings.Contains(output, "DETAILED QUERY LIST - EXECUTION 1") {
		t.Error("Verbose output should contain detailed query list for execution 1")
	}

	if !strings.Contains(output, "DETAILED QUERY LIST - EXECUTION 2") {
		t.Error("Verbose output should contain detailed query list for execution 2")
	}

	if !strings.Contains(output, "SELECT * FROM users WHERE id = $1") {
		t.Error("Verbose output should contain the actual query")
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{10.5, 10.5},
		{-10.5, 10.5},
		{0, 0},
		{-0.001, 0.001},
	}

	for _, tt := range tests {
		result := abs(tt.input)
		if result != tt.expected {
			t.Errorf("abs(%f) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}
