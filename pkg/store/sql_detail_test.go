package store

import (
	"testing"
)

func TestGetSQLQueryDetailByHash(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create a test server
	serverID, err := store.CreateServer(&Server{
		Name: "Test Server",
		URL:  "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create sample queries
	sampleID, err := store.CreateRequest(&SampleQuery{
		Name:        "Test Query",
		ServerID:    ptrUint(uint(serverID)),
		RequestData: `{"query": "query Test { user { id } }"}`,
	})
	if err != nil {
		t.Fatalf("Failed to create sample query: %v", err)
	}

	// Create test executions
	exec1ID, err := store.CreateExecution(&ExecutedRequest{
		SampleID:        ptrUint(uint(sampleID)),
		ServerID:        ptrUint(uint(serverID)),
		RequestIDHeader: "req-1",
		StatusCode:      200,
		DurationMS:      100,
	})
	if err != nil {
		t.Fatalf("Failed to create execution 1: %v", err)
	}

	exec2ID, err := store.CreateExecution(&ExecutedRequest{
		SampleID:        ptrUint(uint(sampleID)),
		ServerID:        ptrUint(uint(serverID)),
		RequestIDHeader: "req-2",
		StatusCode:      200,
		DurationMS:      150,
	})
	if err != nil {
		t.Fatalf("Failed to create execution 2: %v", err)
	}

	// Create SQL queries with the same hash
	testHash := "test-hash-123"
	queries := []SQLQuery{
		{
			ExecutionID:     uint(exec1ID),
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = ?",
			QueryHash:       testHash,
			DurationMS:      10.5,
			QueriedTable:    "users",
			Operation:       "SELECT",
			Rows:            1,
			ExplainPlan:     `[{"Node Type": "Seq Scan"}]`,
		},
		{
			ExecutionID:     uint(exec2ID),
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = ?",
			QueryHash:       testHash,
			DurationMS:      15.5,
			QueriedTable:    "users",
			Operation:       "SELECT",
			Rows:            1,
		},
	}

	err = store.SaveSQLQueries(exec1ID, []SQLQuery{queries[0]})
	if err != nil {
		t.Fatalf("Failed to save SQL query 1: %v", err)
	}

	err = store.SaveSQLQueries(exec2ID, []SQLQuery{queries[1]})
	if err != nil {
		t.Fatalf("Failed to save SQL query 2: %v", err)
	}

	// Test GetSQLQueryDetailByHash
	detail, err := store.GetSQLQueryDetailByHash(testHash)
	if err != nil {
		t.Fatalf("Failed to get SQL query detail: %v", err)
	}

	if detail == nil {
		t.Fatal("Expected SQL query detail, got nil")
	}

	// Verify basic fields
	if detail.QueryHash != testHash {
		t.Errorf("Expected query hash %s, got %s", testHash, detail.QueryHash)
	}

	if detail.Query != "SELECT * FROM users WHERE id = $1" {
		t.Errorf("Expected query 'SELECT * FROM users WHERE id = $1', got %s", detail.Query)
	}

	if detail.NormalizedQuery != "SELECT * FROM users WHERE id = ?" {
		t.Errorf("Expected normalized query 'SELECT * FROM users WHERE id = ?', got %s", detail.NormalizedQuery)
	}

	if detail.Operation != "SELECT" {
		t.Errorf("Expected operation SELECT, got %s", detail.Operation)
	}

	if detail.TableName != "users" {
		t.Errorf("Expected table name users, got %s", detail.TableName)
	}

	// Verify execution count
	if detail.TotalExecutions != 2 {
		t.Errorf("Expected 2 executions, got %d", detail.TotalExecutions)
	}

	// Verify duration statistics
	expectedAvg := (10.5 + 15.5) / 2
	if detail.AvgDuration != expectedAvg {
		t.Errorf("Expected avg duration %f, got %f", expectedAvg, detail.AvgDuration)
	}

	if detail.MinDuration != 10.5 {
		t.Errorf("Expected min duration 10.5, got %f", detail.MinDuration)
	}

	if detail.MaxDuration != 15.5 {
		t.Errorf("Expected max duration 15.5, got %f", detail.MaxDuration)
	}

	// Verify EXPLAIN plan from first query
	if detail.ExplainPlan != `[{"Node Type": "Seq Scan"}]` {
		t.Errorf("Expected EXPLAIN plan to match first query, got %s", detail.ExplainPlan)
	}

	// Verify related executions
	if len(detail.RelatedExecutions) != 2 {
		t.Fatalf("Expected 2 related executions, got %d", len(detail.RelatedExecutions))
	}

	// Verify execution details are present
	foundReq1 := false
	foundReq2 := false
	for _, exec := range detail.RelatedExecutions {
		if exec.RequestIDHeader == "req-1" {
			foundReq1 = true
			if exec.StatusCode != 200 {
				t.Errorf("Expected status code 200 for req-1, got %d", exec.StatusCode)
			}
			if exec.DurationMS != 10.5 {
				t.Errorf("Expected duration 10.5ms for req-1, got %f", exec.DurationMS)
			}
		}
		if exec.RequestIDHeader == "req-2" {
			foundReq2 = true
			if exec.StatusCode != 200 {
				t.Errorf("Expected status code 200 for req-2, got %d", exec.StatusCode)
			}
			if exec.DurationMS != 15.5 {
				t.Errorf("Expected duration 15.5ms for req-2, got %f", exec.DurationMS)
			}
		}
	}

	if !foundReq1 {
		t.Error("Expected to find req-1 in related executions")
	}
	if !foundReq2 {
		t.Error("Expected to find req-2 in related executions")
	}

	// Test non-existent hash
	detail, err = store.GetSQLQueryDetailByHash("non-existent-hash")
	if err != nil {
		t.Fatalf("Failed to get SQL query detail for non-existent hash: %v", err)
	}
	if detail != nil {
		t.Error("Expected nil for non-existent hash")
	}
}

func ptrUint(u uint) *uint {
	return &u
}
