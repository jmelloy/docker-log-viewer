package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"
	"github.com/gorilla/websocket"
)

type WebApp struct {
	docker       *logs.DockerClient
	logs         []logs.LogMessage
	logsMutex    sync.RWMutex
	containers   []logs.Container
	clients      map[*websocket.Conn]bool
	clientsMutex sync.RWMutex
	logChan      chan logs.LogMessage
	ctx          context.Context
	cancel       context.CancelFunc
	upgrader     websocket.Upgrader
	store        *store.Store
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ContainersUpdateMessage struct {
	Containers []logs.Container `json:"containers"`
}

type LogWSMessage struct {
	ContainerID string         `json:"containerId"`
	Timestamp   time.Time      `json:"timestamp"`
	Entry       *logs.LogEntry `json:"entry"`
}

func NewWebApp() (*WebApp, error) {
	docker, err := logs.NewDockerClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Open store
	db, err := store.NewStore("graphql-requests.db")
	if err != nil {
		log.Printf("Warning: Failed to open database: %v", err)
		db = nil
	}

	app := &WebApp{
		docker:  docker,
		logs:    make([]logs.LogMessage, 0),
		clients: make(map[*websocket.Conn]bool),
		logChan: make(chan logs.LogMessage, 1000),
		ctx:     ctx,
		cancel:  cancel,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		store: db,
	}

	return app, nil
}

func (wa *WebApp) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func (wa *WebApp) handleLogs(w http.ResponseWriter, r *http.Request) {
	wa.logsMutex.RLock()
	defer wa.logsMutex.RUnlock()

	startIdx := 0
	if len(wa.logs) > 1000 {
		startIdx = len(wa.logs) - 1000
	}

	logs := make([]LogWSMessage, 0)
	for i := startIdx; i < len(wa.logs); i++ {
		msg := wa.logs[i]
		logs = append(logs, LogWSMessage{
			ContainerID: msg.ContainerID,
			Timestamp:   msg.Timestamp,
			Entry:       msg.Entry,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (wa *WebApp) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wa.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	wa.clientsMutex.Lock()
	wa.clients[conn] = true
	wa.clientsMutex.Unlock()

	defer func() {
		wa.clientsMutex.Lock()
		delete(wa.clients, conn)
		wa.clientsMutex.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (wa *WebApp) broadcastLog(msg logs.LogMessage) {
	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	logMsg := LogWSMessage{
		ContainerID: msg.ContainerID,
		Timestamp:   msg.Timestamp,
		Entry:       msg.Entry,
	}

	wsMsg := WSMessage{
		Type: "log",
	}
	data, _ := json.Marshal(logMsg)
	wsMsg.Data = data

	for client := range wa.clients {
		err := client.WriteJSON(wsMsg)
		if err != nil {
			client.Close()
			delete(wa.clients, client)
		}
	}
}

func (wa *WebApp) processLogs() {
	for {
		select {
		case <-wa.ctx.Done():
			return
		case msg := <-wa.logChan:
			wa.logsMutex.Lock()
			wa.logs = append(wa.logs, msg)
			if len(wa.logs) > 10000 {
				wa.logs = wa.logs[1000:]
			}
			wa.logsMutex.Unlock()

			wa.broadcastLog(msg)
		}
	}
}

func (wa *WebApp) loadContainers() error {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		return err
	}

	wa.containers = containers

	for _, c := range containers {
		if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
			log.Printf("Failed to stream logs for container %s: %v", c.ID, err)
		}
	}

	return nil
}

func (wa *WebApp) monitorContainers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	previousIDs := make(map[string]bool)
	for _, c := range wa.containers {
		previousIDs[c.ID] = true
	}

	for {
		select {
		case <-wa.ctx.Done():
			return
		case <-ticker.C:
			containers, err := wa.docker.ListRunningContainers(wa.ctx)
			if err != nil {
				log.Printf("Failed to list containers: %v", err)
				continue
			}

			currentIDs := make(map[string]bool)
			for _, c := range containers {
				currentIDs[c.ID] = true
			}

			for _, c := range containers {
				if !previousIDs[c.ID] {
					log.Printf("New container detected: %s (%s)", c.ID, c.Name)
					if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
						log.Printf("Failed to stream logs for new container %s: %v", c.ID, err)
					}
				}
			}

			for id := range previousIDs {
				if !currentIDs[id] {
					log.Printf("Container stopped: %s", id)
				}
			}

			if len(containers) != len(wa.containers) || len(currentIDs) != len(previousIDs) {
				wa.containers = containers
				wa.broadcastContainerUpdate(containers)
			}

			previousIDs = currentIDs
		}
	}
}

func (wa *WebApp) broadcastContainerUpdate(containers []logs.Container) {
	update := ContainersUpdateMessage{
		Containers: containers,
	}

	wsMsg := WSMessage{
		Type: "containers",
	}
	data, _ := json.Marshal(update)
	wsMsg.Data = data

	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	for client := range wa.clients {
		err := client.WriteJSON(wsMsg)
		if err != nil {
			client.Close()
			delete(wa.clients, client)
		}
	}
}

func (wa *WebApp) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req sqlexplain.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := sqlexplain.Explain(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Request management handlers
func (wa *WebApp) handleRequests(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		wa.listRequests(w, r)
	case http.MethodPost:
		wa.createRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) listRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := wa.store.ListRequests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (wa *WebApp) createRequest(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming request which may have server fields or a serverID
	var input struct {
		Name        string  `json:"name"`
		ServerID    *uint   `json:"serverId,omitempty"`
		URL         string  `json:"url,omitempty"`
		BearerToken string  `json:"bearerToken,omitempty"`
		DevID       string  `json:"devId,omitempty"`
		RequestData string  `json:"requestData"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If URL is provided but no serverID, create a new server
	var serverID *uint
	if input.ServerID != nil {
		// Use existing server
		serverID = input.ServerID
	} else if input.URL != "" {
		// Create new server with URL and credentials
		server := &store.Server{
			Name:        input.URL, // Use URL as name for now
			URL:         input.URL,
			BearerToken: input.BearerToken,
			DevID:       input.DevID,
		}
		
		sid, err := wa.store.CreateServer(server)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create server: %v", err), http.StatusInternalServerError)
			return
		}
		sidUint := uint(sid)
		serverID = &sidUint
	}

	// Create the request
	req := &store.Request{
		Name:        input.Name,
		ServerID:    serverID,
		RequestData: input.RequestData,
	}

	id, err := wa.store.CreateRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (wa *WebApp) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		req, err := wa.store.GetRequest(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if req == nil {
			http.Error(w, "Request not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(req)
	case http.MethodDelete:
		if err := wa.store.DeleteRequest(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	path = strings.TrimSuffix(path, "/execute")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	// Parse request body for overrides
	var input struct {
		ServerID           *uint  `json:"serverId,omitempty"`
		BearerTokenOverride string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride       string `json:"devIdOverride,omitempty"`
	}
	
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			// Ignore decode errors for backward compatibility
			log.Printf("Warning: failed to decode execute request body: %v", err)
		}
	}

	// Execute request in background with overrides
	go wa.executeRequestWithOverrides(id, input.ServerID, input.BearerTokenOverride, input.DevIDOverride)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (wa *WebApp) handleExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract request ID from query param
	requestIDStr := r.URL.Query().Get("request_id")
	if requestIDStr == "" {
		http.Error(w, "request_id parameter required", http.StatusBadRequest)
		return
	}

	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request_id", http.StatusBadRequest)
		return
	}

	executions, err := wa.store.ListExecutions(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (wa *WebApp) handleAllExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	executions, err := wa.store.ListAllExecutions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (wa *WebApp) handleExecutionDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	detail, err := wa.store.GetExecutionDetail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func (wa *WebApp) handleServers(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	servers, err := wa.store.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func (wa *WebApp) executeRequestWithOverrides(requestID int64, serverIDOverride *uint, bearerTokenOverride, devIDOverride string) {
	req, err := wa.store.GetRequest(requestID)
	if err != nil {
		log.Printf("Failed to get request %d: %v", requestID, err)
		return
	}
	if req == nil {
		log.Printf("Request %d not found", requestID)
		return
	}

	// Generate request ID
	requestIDHeader := generateRequestID()

	// Get server info for execution
	var url, bearerToken, devID string
	var serverIDForExec *uint
	
	// Use override server if provided, otherwise use sample query's server
	if serverIDOverride != nil {
		server, err := wa.store.GetServer(int64(*serverIDOverride))
		if err != nil {
			log.Printf("Failed to get override server %d: %v", *serverIDOverride, err)
			return
		}
		if server != nil {
			url = server.URL
			bearerToken = server.BearerToken
			devID = server.DevID
			serverIDForExec = serverIDOverride
		}
	} else if req.Server != nil {
		url = req.Server.URL
		bearerToken = req.Server.BearerToken
		devID = req.Server.DevID
		serverIDForExec = &req.Server.ID
	}

	// Apply overrides
	if bearerTokenOverride != "" {
		bearerToken = bearerTokenOverride
	}
	if devIDOverride != "" {
		devID = devIDOverride
	}

	execution := &store.Execution{
		RequestID:       uint(requestID),
		ServerID:        serverIDForExec,
		RequestIDHeader: requestIDHeader,
		ExecutedAt:      time.Now(),
	}

	// Execute HTTP request
	startTime := time.Now()
	statusCode, responseBody, responseHeaders, err := makeHTTPRequest(url, []byte(req.RequestData), requestIDHeader, bearerToken, devID)
	execution.DurationMS = time.Since(startTime).Milliseconds()
	execution.StatusCode = statusCode
	execution.ResponseBody = responseBody
	execution.ResponseHeaders = responseHeaders

	if err != nil {
		execution.Error = err.Error()
	}

	// Save execution
	execID, err := wa.store.CreateExecution(execution)
	if err != nil {
		log.Printf("Failed to save execution: %v", err)
		return
	}

	log.Printf("Request %d executed: ID=%s, Status=%d, Duration=%dms", requestID, requestIDHeader, statusCode, execution.DurationMS)

	// Collect logs
	collectedLogs := wa.collectLogsForRequest(requestIDHeader, 10*time.Second)
	log.Printf("Collected %d logs for execution %d", len(collectedLogs), execID)

	// Save logs
	if len(collectedLogs) > 0 {
		if err := wa.store.SaveExecutionLogs(execID, collectedLogs); err != nil {
			log.Printf("Failed to save logs: %v", err)
		}
	}

	// Extract and save SQL queries
	sqlQueries := extractSQLQueries(collectedLogs)
	if len(sqlQueries) > 0 {
		log.Printf("Found %d SQL queries for execution %d", len(sqlQueries), execID)
		if err := wa.store.SaveSQLQueries(execID, sqlQueries); err != nil {
			log.Printf("Failed to save SQL queries: %v", err)
		}
	}
}

func (wa *WebApp) executeRequest(requestID int64) {
	wa.executeRequestWithOverrides(requestID, nil, "", "")
}

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func makeHTTPRequest(url string, data []byte, requestID, bearerToken, devID string) (int, string, string, error) {
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

func (wa *WebApp) collectLogsForRequest(requestID string, timeout time.Duration) []logs.LogMessage {
	collected := []logs.LogMessage{}
	deadline := time.After(timeout)

	for {
		select {
		case msg := <-wa.logChan:
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
						var durationVal float64
						if _, err := strconv.ParseFloat(duration, 64); err == nil {
							durationVal, _ = strconv.ParseFloat(duration, 64)
							query.DurationMS = durationVal
						}
					}
					if table, ok := msg.Entry.Fields["db.table"]; ok {
						query.TableName = table
					}
					if op, ok := msg.Entry.Fields["db.operation"]; ok {
						query.Operation = op
					}
					if rows, ok := msg.Entry.Fields["db.rows"]; ok {
						var rowsVal int
						if _, err := strconv.Atoi(rows); err == nil {
							rowsVal, _ = strconv.Atoi(rows)
							query.Rows = rowsVal
						}
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

func (wa *WebApp) Run(addr string) error {
	if err := wa.loadContainers(); err != nil {
		return err
	}

	// Try to initialize database connection for EXPLAIN queries
	if err := sqlexplain.Init(); err != nil {
		log.Printf("Database connection not available (EXPLAIN feature disabled): %v", err)
	} else {
		log.Printf("Database connection established for EXPLAIN queries")
	}

	go wa.processLogs()
	go wa.monitorContainers()

	http.HandleFunc("/api/containers", wa.handleContainers)
	http.HandleFunc("/api/logs", wa.handleLogs)
	http.HandleFunc("/api/ws", wa.handleWebSocket)
	http.HandleFunc("/api/explain", wa.handleExplain)
	
	// Request management endpoints
	http.HandleFunc("/api/servers", wa.handleServers)
	http.HandleFunc("/api/requests", wa.handleRequests)
	http.HandleFunc("/api/requests/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/execute") {
			wa.handleExecuteRequest(w, r)
		} else {
			wa.handleRequestDetail(w, r)
		}
	})
	http.HandleFunc("/api/executions", wa.handleExecutions)
	http.HandleFunc("/api/all-executions", wa.handleAllExecutions)
	http.HandleFunc("/api/executions/", wa.handleExecutionDetail)
	
	http.Handle("/", http.FileServer(http.Dir("./web")))

	log.Printf("Server starting on http://localhost:%s", addr)
	return http.ListenAndServe(addr, nil)
}

func main() {
	app, err := NewWebApp()
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	defer sqlexplain.Close()
	if app.store != nil {
		defer app.store.Close()
	}

	if err := app.Run(":9000"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
