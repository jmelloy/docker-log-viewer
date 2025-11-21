package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"
)

// ============================================================================
// HTTP Handlers - SQL Analysis & Trace Management
// ============================================================================

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

func (wa *WebApp) handleSaveTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if wa.store == nil {
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
	// Create an execution entry with the trace data

	containerIDs := make([]string, 0, len(input.SelectedContainers))
	for _, container := range wa.containers {
		for _, containerName := range input.SelectedContainers {
			if containerName == wa.containerIDNames[container.ID] {
				containerIDs = append(containerIDs, container.ID)
				break
			}
		}
	}

	logMessages := wa.logStore.Filter(logstore.FilterOptions{
		FieldFilters: fieldFilters,
		ContainerIDs: containerIDs,
	}, 1000)

	messages := make([]logs.LogMessage, 0, len(logMessages))
	for _, msg := range logMessages {
		messages = append(messages, logs.LogMessage{
			Timestamp:   msg.Timestamp,
			ContainerID: msg.ContainerID,
			Entry:       deserializeLogEntry(msg),
		})
	}
	slog.Info("[save trace] found", "count", len(messages), "containerIDs", containerIDs)
	// Create an execution entry with the trace data
	requestIDHeader := input.RequestID
	if requestIDHeader == "" {
		requestIDHeader = input.TraceID
	}
	if requestIDHeader == "" {
		requestIDHeader = input.Name
	}

	// Extract actual request body from logs
	// Look for GraphQL request in logs - typically in fields like "query", "operation", or in message as JSON
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
			// Check if this log has GraphQL query data
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

	id, err := wa.store.CreateExecution(exec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save logs for this execution
	if len(messages) > 0 {
		if err := wa.store.SaveExecutionLogs(id, messages); err != nil {
			slog.Error("failed to save execution logs", "error", err)
		}
	}

	// Extract and save SQL queries from logs (trigger SQL log collection)
	sqlQueries := sqlutil.ExtractSQLQueries(messages)
	if len(sqlQueries) > 0 {
		if err := wa.store.SaveSQLQueries(id, sqlQueries); err != nil {
			slog.Error("failed to save SQL queries from trace", "error", err)
		} else {
			containerIDToConnectionString := map[string]string{}
			buildPortToServerMap := wa.buildPortToServerMap(wa.containers)

			for _, container := range wa.containers {
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
					if err := wa.store.UpdateQueryExplainPlan(id, q.QueryHash, string(planJSON)); err != nil {
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
