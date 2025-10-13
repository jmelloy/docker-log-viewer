package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/store"
)

type Config struct {
	DBPath      string
	URL         string
	DataFile    string
	DataDir     string
	Name        string
	Timeout     time.Duration
	BearerToken string
	DevID       string
	Execute     bool
	List        bool
	Delete      int64
	BatchMode   bool
}

func main() {
	config := parseFlags()

	// Open database
	db, err := store.NewStore(config.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Handle different modes
	if config.List {
		listRequests(db)
		return
	}

	if config.Delete > 0 {
		if err := db.DeleteRequest(config.Delete); err != nil {
			log.Fatalf("Failed to delete request: %v", err)
		}
		log.Printf("Deleted request %d", config.Delete)
		return
	}

	// Handle directory processing mode
	if config.DataDir != "" {
		if err := handleDirectory(db, config); err != nil {
			log.Fatalf("Failed to handle directory: %v", err)
		}
		return
	}

	if config.DataFile == "" || config.URL == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Save or execute request
	if err := handleRequest(db, config); err != nil {
		log.Fatalf("Failed to handle request: %v", err)
	}
}

func parseFlags() Config {
	var config Config

	flag.StringVar(&config.DBPath, "db", "graphql-requests.db", "Path to SQLite database")
	flag.StringVar(&config.URL, "url", "", "GraphQL/API endpoint URL")
	flag.StringVar(&config.DataFile, "data", "", "GraphQL or JSON data file")
	flag.StringVar(&config.DataDir, "dir", "", "Directory containing JSON files to process")
	flag.StringVar(&config.Name, "name", "", "Name for this request (defaults to filename)")
	flag.DurationVar(&config.Timeout, "timeout", 10*time.Second, "Timeout for log collection")
	flag.StringVar(&config.BearerToken, "token", os.Getenv("BEARER_TOKEN"), "Bearer token for authentication")
	flag.StringVar(&config.DevID, "dev-id", os.Getenv("X_GLUE_DEV_USER_ID"), "X-GlueDev-UserID header value")
	flag.BoolVar(&config.Execute, "execute", false, "Execute the request immediately after saving")
	flag.BoolVar(&config.BatchMode, "batch", false, "Execute all requests in batch mode (for directory processing)")
	flag.BoolVar(&config.List, "list", false, "List all saved requests")
	flag.Int64Var(&config.Delete, "delete", 0, "Delete request by ID")

	flag.Parse()
	return config
}

func listRequests(db *store.Store) {
	requests, err := db.ListRequests()
	if err != nil {
		log.Fatalf("Failed to list requests: %v", err)
	}

	if len(requests) == 0 {
		fmt.Println("No saved requests")
		return
	}

	fmt.Printf("Found %d request(s):\n\n", len(requests))
	for _, req := range requests {
		fmt.Printf("ID: %d\n", req.ID)
		fmt.Printf("Name: %s\n", req.Name)
		if req.Server != nil {
			fmt.Printf("Server: %s (%s)\n", req.Server.Name, req.Server.URL)
		} else {
			fmt.Printf("Server: (none)\n")
		}
		fmt.Printf("Created: %s\n", req.CreatedAt.Format(time.RFC3339))

		// Count executions
		executions, _ := db.ListExecutions(int64(req.ID))
		fmt.Printf("Executions: %d\n", len(executions))
		fmt.Println("---")
	}
}

func handleRequest(db *store.Store, config Config) error {
	// Read data file
	data, err := os.ReadFile(config.DataFile)
	if err != nil {
		return fmt.Errorf("failed to read data file: %w", err)
	}

	// Determine name
	name := config.Name
	if name == "" {
		// Use filename without extension
		name = config.DataFile
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if idx := strings.LastIndex(name, "."); idx >= 0 {
			name = name[:idx]
		}
	}

	// Create or get server
	var serverID *uint
	if config.URL != "" {
		server := &store.Server{
			Name:        config.URL, // Use URL as server name for now
			URL:         config.URL,
			BearerToken: config.BearerToken,
			DevID:       config.DevID,
		}

		sid, err := db.CreateServer(server)
		if err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}
		sidUint := uint(sid)
		serverID = &sidUint
	}

	// Create request
	req := &store.SampleQuery{
		Name:        name,
		ServerID:    serverID,
		RequestData: string(data),
	}

	reqID, err := db.CreateRequest(req)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	log.Printf("Saved request '%s' with ID %d", name, reqID)

	// Execute if requested
	if config.Execute {
		log.Printf("Executing request...")
		if err := executeRequest(db, reqID, config); err != nil {
			return fmt.Errorf("failed to execute request: %w", err)
		}
	} else {
		log.Printf("Request saved. Use -execute flag to run it immediately, or use the web UI.")
	}

	return nil
}

func handleDirectory(db *store.Store, config Config) error {
	if config.URL == "" {
		return fmt.Errorf("URL is required when processing a directory")
	}

	// Read all JSON files from the directory
	jsonFiles, err := filepath.Glob(filepath.Join(config.DataDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	if len(jsonFiles) == 0 {
		log.Printf("No JSON files found in directory: %s", config.DataDir)
		return nil
	}

	log.Printf("Found %d JSON files in directory: %s", len(jsonFiles), config.DataDir)

	// Create or get server

	server := &store.Server{
		Name:        config.URL,
		URL:         config.URL,
		BearerToken: config.BearerToken,
		DevID:       config.DevID,
	}

	_, err = db.CreateServer(server)
	if err != nil {
		log.Printf("Failed to create server for file %s: %v", config.DataDir, err)
		panic(err)
	}

	// Process each JSON file
	var requestIDs []int64
	for _, jsonFile := range jsonFiles {
		log.Printf("Processing file: %s", jsonFile)

		// Read the JSON file
		data, err := os.ReadFile(jsonFile)
		if err != nil {
			log.Printf("Failed to read file %s: %v", jsonFile, err)
			continue
		}

		// Parse the JSON to determine if it's a single operation or array
		var operations []struct {
			OperationName string `json:"operationName"`
		}

		// Try to parse as array first
		if err := json.Unmarshal(data, &operations); err != nil {
			// If that fails, try as single operation
			var singleOp struct {
				OperationName string `json:"operationName"`
			}
			if err := json.Unmarshal(data, &singleOp); err != nil {
				log.Printf("Failed to parse JSON in file %s: %v", jsonFile, err)
				continue
			}
			operations = []struct {
				OperationName string `json:"operationName"`
			}{singleOp}
		}

		operationNames := []string{}
		for _, operation := range operations {
			operationNames = append(operationNames, operation.OperationName)
		}

		operationName := strings.Join(operationNames, ":")
		// Process each operation

		// Create request
		req := &store.SampleQuery{
			Name:        operationName,
			ServerID:    &server.ID,
			RequestData: string(data),
		}

		reqID, err := db.CreateRequest(req)
		if err != nil {
			log.Printf("Failed to create request for file %s, operation %s: %v", jsonFile, operationName, err)
			continue
		}

		log.Printf("Saved request '%s' with ID %d", operationName, reqID)
		requestIDs = append(requestIDs, reqID)

	}

	log.Printf("Successfully processed %d files, created %d requests", len(jsonFiles), len(requestIDs))

	// Execute requests if batch mode is enabled
	if config.BatchMode && len(requestIDs) > 0 {
		log.Printf("Executing %d requests in batch mode...", len(requestIDs))
		for i, reqID := range requestIDs {
			log.Printf("Executing request %d/%d (ID: %d)", i+1, len(requestIDs), reqID)
			if err := executeRequest(db, reqID, config); err != nil {
				log.Printf("Failed to execute request %d: %v", reqID, err)
			}
		}
		log.Printf("Batch execution completed")
	} else if len(requestIDs) > 0 {
		log.Printf("Requests saved. Use -batch flag to execute them immediately, or use the web UI.")
	}

	return nil
}

func executeRequest(db *store.Store, requestID int64, config Config) error {
	// Get request details
	req, err := db.GetRequest(requestID)
	if err != nil {
		return fmt.Errorf("failed to get request: %w", err)
	}
	if req == nil {
		return fmt.Errorf("request not found")
	}

	// Create Docker client for log monitoring
	docker, err := logs.NewDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer docker.Close()

	ctx := context.Background()

	// Start log collection
	logChan := make(chan logs.LogMessage, 10000)
	containers, err := docker.ListRunningContainers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		if err := docker.StreamLogs(ctx, c.ID, logChan); err != nil {
			log.Printf("Failed to stream logs for container %s: %v", c.ID, err)
		}
	}

	// Generate request ID
	requestIDHeader := generateRequestID()

	// Get server info for execution
	var url, bearerToken, devID string
	var serverIDForExec *uint
	if req.Server != nil {
		url = req.Server.URL
		bearerToken = req.Server.BearerToken
		devID = req.Server.DevID
		serverIDForExec = &req.Server.ID
	}

	// Execute request
	sampleID := uint(requestID)
	execution := &store.ExecutedRequest{
		SampleID:        &sampleID,
		ServerID:        serverIDForExec,
		RequestIDHeader: requestIDHeader,
		RequestBody:     req.RequestData,
		ExecutedAt:      time.Now(),
	}

	startTime := time.Now()
	statusCode, responseBody, responseHeaders, err := makeRequest(url, []byte(req.RequestData), requestIDHeader, bearerToken, devID)
	execution.DurationMS = time.Since(startTime).Milliseconds()
	execution.StatusCode = statusCode
	execution.ResponseBody = responseBody
	execution.ResponseHeaders = responseHeaders

	if err != nil {
		execution.Error = err.Error()
	}

	// Save execution
	execID, err := db.CreateExecution(execution)
	if err != nil {
		return fmt.Errorf("failed to save execution: %w", err)
	}

	log.Printf("Request ID: %s, Status: %d, Duration: %dms", requestIDHeader, statusCode, execution.DurationMS)

	// Wait a bit for logs to arrive from Docker
	time.Sleep(500 * time.Millisecond)

	// Collect logs
	collectedLogs := collectLogs(requestIDHeader, logChan, config.Timeout)
	log.Printf("Collected %d logs for request %s", len(collectedLogs), requestIDHeader)

	// Save logs
	if len(collectedLogs) > 0 {
		if err := db.SaveExecutionLogs(execID, collectedLogs); err != nil {
			return fmt.Errorf("failed to save logs: %w", err)
		}
	}

	// Extract and save SQL queries
	sqlQueries := extractSQLQueries(collectedLogs)
	if len(sqlQueries) > 0 {
		log.Printf("Found %d SQL queries", len(sqlQueries))
		if err := db.SaveSQLQueries(execID, sqlQueries); err != nil {
			return fmt.Errorf("failed to save SQL queries: %w", err)
		}
	}

	log.Printf("Execution saved with ID %d", execID)
	return nil
}

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func makeRequest(url string, data []byte, requestID, bearerToken, devID string) (int, string, string, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return 0, "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", requestID)

	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if devID != "" {
		req.Header.Set("X-GlueDev-UserID", devID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	// Capture response headers as JSON
	headersJSON, _ := json.Marshal(resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", string(headersJSON), err
	}

	return resp.StatusCode, string(bodyBytes), string(headersJSON), nil
}

func collectLogs(requestID string, logChan <-chan logs.LogMessage, timeout time.Duration) []logs.LogMessage {
	collected := []logs.LogMessage{}
	deadline := time.After(timeout)

	for {
		select {
		case msg := <-logChan:
			if matchesRequestID(msg, requestID) {
				collected = append(collected, msg)
			}
		case <-deadline:
			return collected
		}
	}
}

func matchesRequestID(msg logs.LogMessage, requestID string) bool {
	if msg.Entry == nil || msg.Entry.Fields == nil {
		return false
	}

	for _, field := range []string{"request_id", "requestId", "requestID", "req_id"} {
		if val, ok := msg.Entry.Fields[field]; ok && val == requestID {
			return true
		}
	}

	return false
}

func extractSQLQueries(logMessages []logs.LogMessage) []store.SQLQuery {
	queries := []store.SQLQuery{}

	for _, msg := range logMessages {
		if msg.Entry == nil || msg.Entry.Message == "" {
			continue
		}

		message := msg.Entry.Message
		if strings.Contains(message, "[sql]") {
			sqlMatch := regexp.MustCompile(`\[sql\]:\s*(.+)`).FindStringSubmatch(message)
			if len(sqlMatch) > 1 {
				normalizedQuery := normalizeQuery(sqlMatch[1])
				query := store.SQLQuery{
					Query:           sqlMatch[1],
					NormalizedQuery: normalizedQuery,
					QueryHash:       store.ComputeQueryHash(normalizedQuery),
				}

				if msg.Entry.Fields != nil {
					if duration, ok := msg.Entry.Fields["duration"]; ok {
						fmt.Sscanf(duration, "%f", &query.DurationMS)
					}
					if table, ok := msg.Entry.Fields["db.table"]; ok {
						query.TableName = table
					}
					if op, ok := msg.Entry.Fields["db.operation"]; ok {
						query.Operation = op
					}
					if rows, ok := msg.Entry.Fields["db.rows"]; ok {
						fmt.Sscanf(rows, "%d", &query.Rows)
					}
					// Check both gql.operation and gql.operationName for GraphQL operation
					if gqlOp, ok := msg.Entry.Fields["gql.operation"]; ok {
						query.GraphQLOperation = gqlOp
					} else if gqlOp, ok := msg.Entry.Fields["gql.operationName"]; ok {
						query.GraphQLOperation = gqlOp
					}
				}

				queries = append(queries, query)
			}
		}
	}

	return queries
}

func normalizeQuery(query string) string {
	// Replace numbers with ?
	normalized := regexp.MustCompile(`\b\d+\b`).ReplaceAllString(query, "?")
	// Replace $1, $2, etc. with ?
	normalized = regexp.MustCompile(`\$\d+`).ReplaceAllString(normalized, "?")
	// Replace quoted strings with ?
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "?")
	// Collapse whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}
