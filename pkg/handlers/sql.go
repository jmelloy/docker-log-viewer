package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"
)

// HandleExplain handles SQL EXPLAIN requests
func (wa *WebApp) HandleExplain(w http.ResponseWriter, r *http.Request) {
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

// HandleSaveTrace saves trace data as an execution with logs and SQL queries
func (wa *WebApp) HandleSaveTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if wa.Store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var input struct {
		Name               string             `json:"name"`
		TraceID            string             `json:"traceId"`
		RequestID          string             `json:"requestId"`
		Filters            []TraceFilterValue `json:"filters"`
		SelectedContainers []string           `json:"selectedContainers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fieldFilters := []logstore.FieldFilter{}
	for _, filter := range input.Filters {
		fieldFilters = append(fieldFilters, logstore.FieldFilter{
			Name:  filter.Type,
			Value: filter.Value,
		})
		if filter.Type == "trace_id" && input.TraceID == "" {
			input.TraceID = filter.Value
		}
		if filter.Type == "request_id" && input.RequestID == "" {
			input.RequestID = filter.Value
		}
	}

	slog.Info("[save trace]", "name", input.Name, "traceId", input.TraceID, "requestId", input.RequestID, "filters", input.Filters, "selectedContainers", input.SelectedContainers)

	containerIDs := make([]string, 0, len(input.SelectedContainers))
	for _, container := range wa.Containers {
		for _, containerName := range input.SelectedContainers {
			if containerName == wa.ContainerIDNames[container.ID] {
				containerIDs = append(containerIDs, container.ID)
				break
			}
		}
	}

	logMessages := wa.LogStore.Filter(logstore.FilterOptions{
		FieldFilters: fieldFilters,
		ContainerIDs: containerIDs,
	}, 1000)

	messages := make([]logs.LogMessage, 0, len(logMessages))
	for _, msg := range logMessages {
		messages = append(messages, logs.LogMessage{
			Timestamp:   msg.Timestamp,
			ContainerID: msg.ContainerID,
			Entry:       DeserializeLogEntry(msg),
		})
	}
	slog.Info("[save trace] found", "count", len(messages), "containerIDs", containerIDs)

	requestIDHeader := input.RequestID
	if requestIDHeader == "" {
		requestIDHeader = input.TraceID
	}
	if requestIDHeader == "" {
		requestIDHeader = input.Name
	}

	// Extract actual request body from logs
	requestBody := ""
	minTimestamp := time.Now().Add(1 * time.Hour)
	maxTimestamp := time.Time{}

	statusCode := 200
	for _, logMsg := range logMessages {
		if logMsg.Timestamp.Before(minTimestamp) {
			minTimestamp = logMsg.Timestamp
		}
		if logMsg.Timestamp.After(maxTimestamp) {
			maxTimestamp = logMsg.Timestamp
		}
		if logMsg.Fields != nil {
			if query, ok := logMsg.Fields["Operations"]; ok {
				requestBody = query
			}

			if status, ok := logMsg.Fields["status"]; ok {
				statusCodeVal, err := strconv.Atoi(status)
				if err != nil {
					slog.Error("failed to parse status", "error", err)
				}
				statusCode = statusCodeVal
			}
		}
	}

	// Calculate duration from logs
	var durationMS int64
	if len(messages) > 1 {
		if !minTimestamp.IsZero() && !maxTimestamp.IsZero() {
			durationMS = maxTimestamp.Sub(minTimestamp).Milliseconds()
		}
	}

	exec := &store.ExecutedRequest{
		RequestIDHeader: requestIDHeader,
		RequestBody:     requestBody,
		StatusCode:      statusCode,
		DurationMS:      durationMS,
		ExecutedAt:      time.Now(),
		Name:            input.Name,
	}

	id, err := wa.Store.CreateExecution(exec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save logs for this execution
	if len(messages) > 0 {
		if err := wa.Store.SaveExecutionLogs(id, messages); err != nil {
			slog.Error("failed to save execution logs", "error", err)
		}
	}

	// Extract and save SQL queries from logs
	sqlQueries := sqlutil.ExtractSQLQueries(messages)
	if len(sqlQueries) > 0 {
		if err := wa.Store.SaveSQLQueries(id, sqlQueries); err != nil {
			slog.Error("failed to save SQL queries from trace", "error", err)
		} else {
			containerIDToConnectionString := map[string]string{}
			buildPortToServerMap := wa.BuildPortToServerMap(wa.Containers)

			for _, container := range wa.Containers {
				if slices.Contains(containerIDs, container.ID) {
					if len(container.Ports) > 0 {
						for _, port := range container.Ports {
							if buildPortToServerMap[port.PublicPort] != "" {
								containerIDToConnectionString[container.ID] = buildPortToServerMap[port.PublicPort]
								break
							}
						}
					}
				}
			}

			// Auto-execute EXPLAIN for queries taking longer than 2ms
			for i, q := range sqlQueries {
				connectionString := containerIDToConnectionString[q.ContainerID]
				if q.DurationMS > 2.0 && connectionString != "" {
					// Parse db.vars to extract parameters
					variables := make(map[string]string)
					if q.Variables != "" {
						var varsArray []interface{}
						if err := json.Unmarshal([]byte(q.Variables), &varsArray); err == nil {
							// Convert array values to map with 1-based indices
							for idx, val := range varsArray {
								variables[fmt.Sprintf("%d", idx+1)] = fmt.Sprintf("%v", val)
							}
						} else {
							slog.Warn("failed to parse db.vars", "query_index", i, "error", err)
						}
					}

					// Execute EXPLAIN
					req := sqlexplain.Request{
						Query:            q.Query,
						Variables:        variables,
						ConnectionString: connectionString,
					}
					resp := sqlexplain.Explain(req)

					if resp.Error != "" {
						slog.Warn("auto-EXPLAIN failed", "query_index", i, "error", resp.Error)
						continue
					}

					// Save the EXPLAIN plan to database
					planJSON, _ := json.Marshal(resp.QueryPlan)
					if err := wa.Store.UpdateQueryExplainPlan(id, q.QueryHash, string(planJSON)); err != nil {
						slog.Error("failed to save EXPLAIN plan", "query_index", i, "error", err)
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"message": "Trace saved successfully as execution",
	})
}

// HandleExecute executes a request without requiring a saved sample query
func (wa *WebApp) HandleExecute(w http.ResponseWriter, r *http.Request) {
	if wa.Store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		ServerID            *uint  `json:"serverId"`
		URLOverride         string `json:"urlOverride,omitempty"`
		BearerTokenOverride string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride       string `json:"devIdOverride,omitempty"`
		RequestData         string `json:"requestData"`
		Sync                bool   `json:"sync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if input.RequestData == "" {
		http.Error(w, "requestData is required", http.StatusBadRequest)
		return
	}

	if input.ServerID == nil {
		http.Error(w, "serverId is required", http.StatusBadRequest)
		return
	}

	// Get server info
	server, err := wa.Store.GetServer(int64(*input.ServerID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if server == nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	url := server.URL
	bearerToken := server.BearerToken
	devID := server.DevID
	experimentalMode := server.ExperimentalMode
	connectionString := ""
	if server.DefaultDatabase != nil {
		connectionString = server.DefaultDatabase.ConnectionString
	}

	// Apply overrides
	if input.URLOverride != "" {
		url = input.URLOverride
	}
	if input.BearerTokenOverride != "" {
		bearerToken = input.BearerTokenOverride
	}
	if input.DevIDOverride != "" {
		devID = input.DevIDOverride
	}

	// Generate request ID
	requestIDHeader := httputil.GenerateRequestID()

	// Create execution record immediately with pending status
	execution := &store.ExecutedRequest{
		ServerID:            input.ServerID,
		RequestIDHeader:     requestIDHeader,
		RequestBody:         input.RequestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0, // 0 indicates pending
		IsSync:              input.Sync,
		BearerTokenOverride: input.BearerTokenOverride,
		DevIDOverride:       input.DevIDOverride,
	}

	// Save execution immediately
	execID, err := wa.Store.CreateExecution(execution)
	if err != nil {
		slog.Error("failed to save execution", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Define execution logic as a function
	executeRequest := func() {
		startTime := time.Now()
		statusCode, responseBody, responseHeaders, err := httputil.MakeHTTPRequest(url, []byte(input.RequestData), requestIDHeader, bearerToken, devID, experimentalMode)
		execution.DurationMS = time.Since(startTime).Milliseconds()
		execution.StatusCode = statusCode
		execution.ResponseBody = responseBody
		execution.ResponseHeaders = responseHeaders

		if err != nil {
			execution.Error = err.Error()
		}

		// Check for GraphQL errors in response body (even with 200 status)
		if execution.Error == "" && statusCode == 200 && responseBody != "" {
			var responseData interface{}
			if err := json.Unmarshal([]byte(responseBody), &responseData); err == nil {
				if hasErrors, message, key := httputil.ContainsErrorsKey(responseData, ""); hasErrors {
					slog.Warn("GraphQL errors in response", "message", message, "key", key)
					msg := fmt.Sprintf("GraphQL errors: %s", message)
					if key != "" {
						msg += fmt.Sprintf(" at %s", key)
					}
					execution.Error = msg
				}
			}
		}

		// Update execution with results
		execution.ID = uint(execID)
		if err := wa.Store.UpdateExecution(execution); err != nil {
			slog.Error("failed to update execution", "error", err)
			return
		}

		slog.Info("request executed", "header_id", requestIDHeader, "status", statusCode, "duration_ms", execution.DurationMS)

		// Collect logs
		collectedLogs := httputil.CollectLogsForRequest(requestIDHeader, wa.LogStore, 10*time.Second)

		// Save logs
		if len(collectedLogs) > 0 {
			if err := wa.Store.SaveExecutionLogs(execID, collectedLogs); err != nil {
				slog.Error("failed to save logs", "error", err)
			}
		}

		// Extract and save SQL queries
		sqlQueries := sqlutil.ExtractSQLQueries(collectedLogs)
		if len(sqlQueries) > 0 {
			if err := wa.Store.SaveSQLQueries(execID, sqlQueries); err != nil {
				slog.Error("failed to save SQL queries", "error", err)
			}

			// Auto-execute EXPLAIN for queries taking longer than 2ms
			for i, q := range sqlQueries {
				if q.DurationMS > 2.0 {
					// Parse db.vars to extract parameters
					variables := make(map[string]string)
					if q.Variables != "" {
						var varsArray []interface{}
						if err := json.Unmarshal([]byte(q.Variables), &varsArray); err == nil {
							// Convert array values to map with 1-based indices
							for idx, val := range varsArray {
								variables[fmt.Sprintf("%d", idx+1)] = fmt.Sprintf("%v", val)
							}
						} else {
							slog.Warn("failed to parse db.vars", "query_index", i, "error", err)
						}
					}

					// Execute EXPLAIN
					req := sqlexplain.Request{
						Query:            q.Query,
						Variables:        variables,
						ConnectionString: connectionString,
					}
					resp := sqlexplain.Explain(req)

					if resp.Error != "" {
						slog.Warn("auto-EXPLAIN failed", "query_index", i, "error", resp.Error)
						continue
					}

					// Save the EXPLAIN plan to database
					planJSON, _ := json.Marshal(resp.QueryPlan)
					if err := wa.Store.UpdateQueryExplainPlan(execID, q.QueryHash, string(planJSON)); err != nil {
						slog.Error("failed to save EXPLAIN plan", "query_index", i, "error", err)
					}
				}
			}
		}
	}

	// If sync is true, execute synchronously and return response
	if input.Sync {
		executeRequest()

		// Return the execution result with response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "completed",
			"executionId":  execID,
			"responseBody": execution.ResponseBody,
			"statusCode":   execution.StatusCode,
			"durationMs":   execution.DurationMS,
			"error":        execution.Error,
		})
	} else {
		// Execute HTTP request in background
		go executeRequest()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "started",
			"executionId": execID,
		})
	}
}
