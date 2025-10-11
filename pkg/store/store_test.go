package store

import (
	"os"
	"testing"
	"time"

	"docker-log-parser/pkg/logs"
)

func TestStore(t *testing.T) {
	// Create temporary database
	dbPath := "/tmp/test_store.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test creating a request
	req := &Request{
		Name:        "Test Request",
		URL:         "https://api.example.com/graphql",
		RequestData: `{"query": "{ test }"}`,
		BearerToken: "test-token",
		DevID:       "test-dev-id",
	}

	reqID, err := store.CreateRequest(req)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Test retrieving request
	retrieved, err := store.GetRequest(reqID)
	if err != nil {
		t.Fatalf("Failed to get request: %v", err)
	}
	if retrieved.Name != req.Name {
		t.Errorf("Expected name %s, got %s", req.Name, retrieved.Name)
	}

	// Test listing requests
	requests, err := store.ListRequests()
	if err != nil {
		t.Fatalf("Failed to list requests: %v", err)
	}
	if len(requests) != 1 {
		t.Errorf("Expected 1 request, got %d", len(requests))
	}

	// Test creating execution
	exec := &Execution{
		RequestID:       reqID,
		RequestIDHeader: "test-req-id",
		StatusCode:      200,
		DurationMS:      150,
		ResponseBody:    `{"data": {}}`,
	}

	execID, err := store.CreateExecution(exec)
	if err != nil {
		t.Fatalf("Failed to create execution: %v", err)
	}

	// Test retrieving execution
	retrievedExec, err := store.GetExecution(execID)
	if err != nil {
		t.Fatalf("Failed to get execution: %v", err)
	}
	if retrievedExec.StatusCode != exec.StatusCode {
		t.Errorf("Expected status code %d, got %d", exec.StatusCode, retrievedExec.StatusCode)
	}

	// Test saving logs
	logMessages := []logs.LogMessage{
		{
			ContainerID: "container1",
			Timestamp:   time.Now(),
			Entry: &logs.LogEntry{
				Level:   "INFO",
				Message: "Test log message",
				Raw:     "INFO Test log message",
				Fields:  map[string]string{"key": "value"},
			},
		},
	}

	err = store.SaveExecutionLogs(execID, logMessages)
	if err != nil {
		t.Fatalf("Failed to save execution logs: %v", err)
	}

	// Test retrieving logs
	retrievedLogs, err := store.GetExecutionLogs(execID)
	if err != nil {
		t.Fatalf("Failed to get execution logs: %v", err)
	}
	if len(retrievedLogs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(retrievedLogs))
	}

	// Test saving SQL queries
	sqlQueries := []SQLQuery{
		{
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = ?",
			DurationMS:      15.5,
			TableName:       "users",
			Operation:       "SELECT",
			Rows:            1,
		},
	}

	err = store.SaveSQLQueries(execID, sqlQueries)
	if err != nil {
		t.Fatalf("Failed to save SQL queries: %v", err)
	}

	// Test retrieving SQL queries
	retrievedQueries, err := store.GetSQLQueries(execID)
	if err != nil {
		t.Fatalf("Failed to get SQL queries: %v", err)
	}
	if len(retrievedQueries) != 1 {
		t.Errorf("Expected 1 query, got %d", len(retrievedQueries))
	}

	// Test execution detail
	detail, err := store.GetExecutionDetail(execID)
	if err != nil {
		t.Fatalf("Failed to get execution detail: %v", err)
	}
	if detail.Execution.ID != execID {
		t.Errorf("Expected execution ID %d, got %d", execID, detail.Execution.ID)
	}
	if len(detail.Logs) != 1 {
		t.Errorf("Expected 1 log in detail, got %d", len(detail.Logs))
	}
	if len(detail.SQLQueries) != 1 {
		t.Errorf("Expected 1 SQL query in detail, got %d", len(detail.SQLQueries))
	}
	if detail.SQLAnalysis == nil {
		t.Error("Expected SQL analysis to be present")
	}

	// Test deleting request (should cascade)
	err = store.DeleteRequest(reqID)
	if err != nil {
		t.Fatalf("Failed to delete request: %v", err)
	}

	// Verify deletion
	requests, err = store.ListRequests()
	if err != nil {
		t.Fatalf("Failed to list requests after delete: %v", err)
	}
	if len(requests) != 0 {
		t.Errorf("Expected 0 requests after delete, got %d", len(requests))
	}
}
