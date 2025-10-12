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

	// Test creating a server
	server := &Server{
		Name:        "Test Server",
		URL:         "https://api.example.com/graphql",
		BearerToken: "test-token",
		DevID:       "test-dev-id",
	}

	serverID, err := store.CreateServer(server)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test retrieving server
	retrievedServer, err := store.GetServer(serverID)
	if err != nil {
		t.Fatalf("Failed to get server: %v", err)
	}
	if retrievedServer.Name != server.Name {
		t.Errorf("Expected server name %s, got %s", server.Name, retrievedServer.Name)
	}

	// Test listing servers
	servers, err := store.ListServers()
	if err != nil {
		t.Fatalf("Failed to list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	// Test creating a request
	serverIDUint := uint(serverID)
	req := &Request{
		Name:        "Test Request",
		ServerID:    &serverIDUint,
		RequestData: `{"query": "{ test }"}`,
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
	if retrieved.Server == nil {
		t.Error("Expected server to be preloaded")
	} else if retrieved.Server.URL != server.URL {
		t.Errorf("Expected server URL %s, got %s", server.URL, retrieved.Server.URL)
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
		RequestID:       uint(reqID),
		ServerID:        &serverIDUint,
		RequestIDHeader: "test-req-id",
		StatusCode:      200,
		DurationMS:      150,
		ResponseBody:    `{"data": {}}`,
		ResponseHeaders: `{"content-type": "application/json"}`,
		ExecutedAt:      time.Now(),
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
	normalizedQuery := "SELECT * FROM users WHERE id = ?"
	sqlQueries := []SQLQuery{
		{
			Query:            "SELECT * FROM users WHERE id = $1",
			NormalizedQuery:  normalizedQuery,
			QueryHash:        ComputeQueryHash(normalizedQuery),
			DurationMS:       15.5,
			TableName:        "users",
			Operation:        "SELECT",
			Rows:             1,
			GraphQLOperation: "FetchUsers",
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
	if retrievedQueries[0].GraphQLOperation != "FetchUsers" {
		t.Errorf("Expected GraphQL operation 'FetchUsers', got '%s'", retrievedQueries[0].GraphQLOperation)
	}

	// Test execution detail
	detail, err := store.GetExecutionDetail(execID)
	if err != nil {
		t.Fatalf("Failed to get execution detail: %v", err)
	}
	if detail.Execution.ID != uint(execID) {
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

func TestDatabaseURL(t *testing.T) {
	// Create temporary database
	dbPath := "/tmp/test_database_url.db"
	defer os.Remove(dbPath)

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Test creating a database URL
	dbURL := &DatabaseURL{
		Name:             "Production DB",
		ConnectionString: "postgresql://user:pass@localhost:5432/prod",
		DatabaseType:     "postgresql",
	}

	dbURLID, err := store.CreateDatabaseURL(dbURL)
	if err != nil {
		t.Fatalf("Failed to create database URL: %v", err)
	}

	// Test retrieving database URL
	retrievedDBURL, err := store.GetDatabaseURL(dbURLID)
	if err != nil {
		t.Fatalf("Failed to get database URL: %v", err)
	}
	if retrievedDBURL.Name != dbURL.Name {
		t.Errorf("Expected database URL name %s, got %s", dbURL.Name, retrievedDBURL.Name)
	}
	if retrievedDBURL.ConnectionString != dbURL.ConnectionString {
		t.Errorf("Expected connection string %s, got %s", dbURL.ConnectionString, retrievedDBURL.ConnectionString)
	}

	// Test listing database URLs
	dbURLs, err := store.ListDatabaseURLs()
	if err != nil {
		t.Fatalf("Failed to list database URLs: %v", err)
	}
	if len(dbURLs) != 1 {
		t.Errorf("Expected 1 database URL, got %d", len(dbURLs))
	}

	// Test updating database URL
	retrievedDBURL.Name = "Updated Production DB"
	err = store.UpdateDatabaseURL(retrievedDBURL)
	if err != nil {
		t.Fatalf("Failed to update database URL: %v", err)
	}

	updatedDBURL, err := store.GetDatabaseURL(dbURLID)
	if err != nil {
		t.Fatalf("Failed to get updated database URL: %v", err)
	}
	if updatedDBURL.Name != "Updated Production DB" {
		t.Errorf("Expected updated name, got %s", updatedDBURL.Name)
	}

	// Test creating server with default database
	dbURLIDUint := uint(dbURLID)
	server := &Server{
		Name:              "Test Server",
		URL:               "https://api.example.com/graphql",
		BearerToken:       "test-token",
		DefaultDatabaseID: &dbURLIDUint,
	}

	serverID, err := store.CreateServer(server)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test retrieving server with preloaded default database
	retrievedServer, err := store.GetServer(serverID)
	if err != nil {
		t.Fatalf("Failed to get server: %v", err)
	}
	if retrievedServer.DefaultDatabaseID == nil {
		t.Error("Expected server to have a default database ID")
	} else if *retrievedServer.DefaultDatabaseID != dbURLIDUint {
		t.Errorf("Expected default database ID %d, got %d", dbURLIDUint, *retrievedServer.DefaultDatabaseID)
	}

	// Test deleting database URL
	err = store.DeleteDatabaseURL(dbURLID)
	if err != nil {
		t.Fatalf("Failed to delete database URL: %v", err)
	}

	// Verify deletion
	dbURLs, err = store.ListDatabaseURLs()
	if err != nil {
		t.Fatalf("Failed to list database URLs after delete: %v", err)
	}
	if len(dbURLs) != 0 {
		t.Errorf("Expected 0 database URLs after delete, got %d", len(dbURLs))
	}
}

