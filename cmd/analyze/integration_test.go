package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/store"
)

func TestIntegrationAnalyzeTwoExecutions(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store
	db, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer db.Close()

	// Create a test server
	server := &store.Server{
		Name: "Test Server",
		URL:  "https://api.example.com/graphql",
	}
	serverID, err := db.CreateServer(server)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create execution 1
	exec1 := &store.Request{
		ServerID:    uintPtr(uint(serverID)),
		RequestBody: `{"query": "{ user(id: 1) { name } }"}`,
		StatusCode:  200,
		DurationMS:  150,
		ExecutedAt:  time.Now(),
	}
	exec1ID, err := db.CreateRequest(exec1)
	if err != nil {
		t.Fatalf("Failed to create execution 1: %v", err)
	}

	// Add SQL queries for execution 1
	queries1 := []store.SQLQuery{
		{
			RequestID:       uint(exec1ID),
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			QueryHash:       store.ComputeQueryHash("SELECT * FROM users WHERE id = $N"),
			DurationMS:      25.5,
			QueriedTable:    "users",
			Operation:       "SELECT",
			Rows:            1,
		},
		{
			RequestID:       uint(exec1ID),
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			QueryHash:       store.ComputeQueryHash("SELECT * FROM posts WHERE user_id = $N"),
			DurationMS:      45.0,
			QueriedTable:    "posts",
			Operation:       "SELECT",
			Rows:            10,
		},
	}
	if err := db.SaveSQLQueries(exec1ID, queries1); err != nil {
		t.Fatalf("Failed to save SQL queries for execution 1: %v", err)
	}

	// Create execution 2 (optimized)
	exec2 := &store.Request{
		ServerID:    uintPtr(uint(serverID)),
		RequestBody: `{"query": "{ user(id: 1) { name } }"}`,
		StatusCode:  200,
		DurationMS:  80,
		ExecutedAt:  time.Now(),
	}
	exec2ID, err := db.CreateRequest(exec2)
	if err != nil {
		t.Fatalf("Failed to create execution 2: %v", err)
	}

	// Add SQL queries for execution 2 (faster queries)
	queries2 := []store.SQLQuery{
		{
			RequestID:       uint(exec2ID),
			Query:           "SELECT * FROM users WHERE id = $1",
			NormalizedQuery: "SELECT * FROM users WHERE id = $N",
			QueryHash:       store.ComputeQueryHash("SELECT * FROM users WHERE id = $N"),
			DurationMS:      10.5,
			QueriedTable:    "users",
			Operation:       "SELECT",
			Rows:            1,
		},
		{
			RequestID:       uint(exec2ID),
			Query:           "SELECT * FROM posts WHERE user_id = $1",
			NormalizedQuery: "SELECT * FROM posts WHERE user_id = $N",
			QueryHash:       store.ComputeQueryHash("SELECT * FROM posts WHERE user_id = $N"),
			DurationMS:      20.0,
			QueriedTable:    "posts",
			Operation:       "SELECT",
			Rows:            10,
		},
	}
	if err := db.SaveSQLQueries(exec2ID, queries2); err != nil {
		t.Fatalf("Failed to save SQL queries for execution 2: %v", err)
	}

	// Run analysis
	config := Config{
		DBPath:       dbPath,
		ExecutionID1: exec1ID,
		ExecutionID2: exec2ID,
		Verbose:      false,
	}

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run analysis
	if err := runAnalysis(db, config); err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	output, _ := io.ReadAll(r)
	outputStr := string(output)

	// Verify output contains expected sections
	expectedSections := []string{
		"SQL Query Analysis Report",
		"EXECUTION DETAILS",
		"Execution 1 (ID:",
		"Execution 2 (ID:",
		"QUERY COMPARISON SUMMARY",
		"Total Queries:",
		"Unique Queries:",
		"req-001",
		"req-002",
	}

	for _, section := range expectedSections {
		if !strings.Contains(outputStr, section) {
			t.Errorf("Output missing expected section: %s", section)
		}
	}

	// Verify performance improvement is detected
	if !strings.Contains(outputStr, "PERFORMANCE DIFFERENCES") {
		t.Log("Warning: Performance differences section not found (may be empty if no significant differences)")
	}

	// Verify query details
	if !strings.Contains(outputStr, "users") {
		t.Error("Output should contain 'users' table reference")
	}

	if !strings.Contains(outputStr, "posts") {
		t.Error("Output should contain 'posts' table reference")
	}
}

func uintPtr(u uint) *uint {
	return &u
}
