package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"
)

type Config struct {
	DBPath           string
	URL              string
	DataFile         string
	DataDir          string
	Name             string
	Timeout          time.Duration
	BearerToken      string
	DevID            string
	ExperimentalMode string
	Execute          bool
	List             bool
	Delete           int64
	BatchMode        bool
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("INFO: ")

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
		if err := db.DeleteSampleQuery(config.Delete); err != nil {
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
	flag.StringVar(&config.URL, "url", "http://localhost:8080/graphql", "GraphQL/API endpoint URL")
	flag.StringVar(&config.DataFile, "data", "", "GraphQL or JSON data file")
	flag.StringVar(&config.DataDir, "dir", "", "Directory containing JSON files to process")
	flag.StringVar(&config.Name, "name", "", "Name for this request (defaults to filename)")
	flag.DurationVar(&config.Timeout, "timeout", 10*time.Second, "Timeout for log collection")
	flag.StringVar(&config.BearerToken, "token", os.Getenv("BEARER_TOKEN"), "Bearer token for authentication")
	flag.StringVar(&config.DevID, "dev-id", os.Getenv("X_GLUE_DEV_USER_ID"), "X-GlueDev-UserID header value")
	flag.StringVar(&config.ExperimentalMode, "experimental", os.Getenv("X_GLUE_EXPERIMENTAL_MODE"), "x-glue-experimental-mode header value")
	flag.BoolVar(&config.Execute, "execute", true, "Execute the request immediately (default: true)")
	flag.BoolVar(&config.BatchMode, "batch", false, "Execute all requests in batch mode (for directory processing)")
	flag.BoolVar(&config.List, "list", false, "List all saved requests")
	flag.Int64Var(&config.Delete, "delete", 0, "Delete request by ID")

	flag.Parse()
	return config
}

func listRequests(db *store.Store) {
	requests, err := db.ListSampleQueries()
	if err != nil {
		log.Fatalf("Failed to list requests: %v", err)
	}

	if len(requests) == 0 {
		log.Println("No saved requests")
		return
	}

	log.Printf("Found %d request(s):\n\n", len(requests))
	for _, req := range requests {
		log.Printf("ID: %d", req.ID)
		log.Printf("Name: %s", req.Name)
		if req.Server != nil {
			log.Printf("Server: %s (%s)", req.Server.Name, req.Server.URL)
		} else {
			log.Printf("Server: (none)")
		}
		log.Printf("Created: %s", req.CreatedAt.Format(time.RFC3339))

		// Count executions
		executions, _ := db.ListRequestsBySample(int64(req.ID))
		log.Printf("Executions: %d", len(executions))
		log.Println("---")
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
			Name:             config.URL, // Use URL as server name for now
			URL:              config.URL,
			BearerToken:      config.BearerToken,
			DevID:            config.DevID,
			ExperimentalMode: config.ExperimentalMode,
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

	reqID, err := db.CreateSampleQuery(req)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute if requested (default is true)
	if config.Execute {
		log.Printf("Executing request '%s' with ID %d...", name, reqID)
		if err := executeRequest(db, reqID, config); err != nil {
			return fmt.Errorf("failed to execute request: %w", err)
		}
	} else {
		log.Printf("Saved request '%s' with ID %d (not executing, use -execute=true to run)", name, reqID)
	}

	return nil
}

func handleDirectory(db *store.Store, config Config) error {
	// if config.URL == "" {
	// 	return fmt.Errorf("URL is required when processing a directory")
	// }

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
				log.Printf("failed to parse JSON in file %s: %v", jsonFile, err)
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
			RequestData: string(data),
		}

		reqID, err := db.CreateSampleQuery(req)
		if err != nil {
			log.Printf("failed to create request for file %s, operation %s: %v", jsonFile, operationName, err)
			continue
		}

		log.Printf("saved request '%s' with ID %d", operationName, reqID)
		requestIDs = append(requestIDs, reqID)

	}

	log.Printf("successfully processed %d files, created %d requests", len(jsonFiles), len(requestIDs))

	// Execute requests if batch mode is enabled
	if config.BatchMode && len(requestIDs) > 0 {
		log.Printf("executing %d requests in batch mode", len(requestIDs))
		for i, reqID := range requestIDs {
			log.Printf("executing request %d/%d (ID: %d)", i+1, len(requestIDs), reqID)
			if err := executeRequest(db, reqID, config); err != nil {
				log.Printf("failed to execute request %d: %v", reqID, err)
			}
		}
		log.Printf("batch execution completed")
	} else if len(requestIDs) > 0 {
		log.Printf("requests saved; use -batch flag to execute them immediately, or use the web UI")
	}

	return nil
}

func executeRequest(db *store.Store, requestID int64, config Config) error {
	// Get request details
	req, err := db.GetSampleQuery(requestID)
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
		if err := docker.StreamLogs(ctx, c.ID, logChan, nil); err != nil {
			log.Printf("failed to stream logs for container %s: %v", c.ID, err)
		}
	}

	// Generate request ID
	requestIDHeader := httputil.GenerateRequestID()

	// Get server info for execution
	var url, bearerToken, devID, experimentalMode string
	var serverIDForExec *uint
	if req.Server != nil {
		url = req.Server.URL
		bearerToken = req.Server.BearerToken
		devID = req.Server.DevID
		experimentalMode = req.Server.ExperimentalMode
		serverIDForExec = &req.Server.ID
	}

	// Execute request
	sampleID := uint(requestID)
	execution := &store.Request{
		SampleID:    &sampleID,
		ServerID:    serverIDForExec,
		RequestBody: req.RequestData,
		ExecutedAt:  time.Now(),
	}

	startTime := time.Now()
	statusCode, responseBody, responseHeaders, err := httputil.MakeHTTPRequest(url, []byte(req.RequestData), requestIDHeader, bearerToken, devID, experimentalMode)
	execution.DurationMS = time.Since(startTime).Milliseconds()
	execution.StatusCode = statusCode
	execution.ResponseBody = responseBody
	execution.ResponseHeaders = responseHeaders

	if err != nil {
		execution.Error = err.Error()
	}

	// Save execution
	execID, err := db.CreateRequest(execution)
	if err != nil {
		return fmt.Errorf("failed to save execution: %w", err)
	}

	// Wait a bit for logs to arrive from Docker
	time.Sleep(500 * time.Millisecond)

	// Collect logs
	collectedLogs := collectLogs(requestIDHeader, logChan, config.Timeout)

	// Save logs
	if len(collectedLogs) > 0 {
		if err := db.SaveRequestLogs(execID, collectedLogs); err != nil {
			return fmt.Errorf("failed to save logs: %w", err)
		}
	}

	// Extract and save SQL queries
	sqlQueries := sqlutil.ExtractSQLQueries(collectedLogs)
	if len(sqlQueries) > 0 {
		if err := db.SaveSQLQueries(execID, sqlQueries); err != nil {
			return fmt.Errorf("failed to save SQL queries: %w", err)
		}
	}

	return nil
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
