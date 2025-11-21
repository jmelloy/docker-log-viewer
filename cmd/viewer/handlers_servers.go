package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"
)

// ============================================================================
// HTTP Handlers - Server & Database Configuration
// ============================================================================

func (wa *WebApp) handleServers(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		servers, err := wa.store.ListServers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(servers)
	case http.MethodPost:
		var server store.Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := wa.store.CreateServer(&server)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"id": id})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleServerDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/servers/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid server ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		server, err := wa.store.GetServer(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if server == nil {
			http.Error(w, "Server not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(server)
	case http.MethodPut:
		var server store.Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		server.ID = uint(id)
		if err := wa.store.UpdateServer(&server); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := wa.store.DeleteServer(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleDatabaseURLs(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		wa.listDatabaseURLs(w, r)
	case http.MethodPost:
		wa.createDatabaseURL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) listDatabaseURLs(w http.ResponseWriter, r *http.Request) {
	dbURLs, err := wa.store.ListDatabaseURLs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dbURLs)
}

func (wa *WebApp) createDatabaseURL(w http.ResponseWriter, r *http.Request) {
	var dbURL store.DatabaseURL
	if err := json.NewDecoder(r.Body).Decode(&dbURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := wa.store.CreateDatabaseURL(&dbURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (wa *WebApp) handleDatabaseURLDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/database-urls/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid database URL ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		dbURL, err := wa.store.GetDatabaseURL(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if dbURL == nil {
			http.Error(w, "Database URL not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbURL)
	case http.MethodPut:
		var dbURL store.DatabaseURL
		if err := json.NewDecoder(r.Body).Decode(&dbURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dbURL.ID = uint(id)
		if err := wa.store.UpdateDatabaseURL(&dbURL); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := wa.store.DeleteDatabaseURL(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) executeRequestWithOverrides(requestID int64, serverIDOverride *uint, urlOverride, bearerTokenOverride, devIDOverride, requestDataOverride string) int64 {
	req, err := wa.store.GetRequest(requestID)
	if err != nil {
		slog.Error("failed to get request", "request_id", requestID, "error", err)
		return 0
	}
	if req == nil {
		slog.Warn("request not found", "request_id", requestID)
		return 0
	}

	// Generate request ID
	requestIDHeader := httputil.GenerateRequestID()

	// Get server info for execution
	var url, bearerToken, devID, experimentalMode, connectionString string
	var serverIDForExec *uint

	// Use override server if provided, otherwise use sample query's server
	if serverIDOverride != nil {
		server, err := wa.store.GetServer(int64(*serverIDOverride))
		if err != nil {
			slog.Error("failed to get override server", "server_id", *serverIDOverride, "error", err)
			return 0
		}
		if server != nil {
			url = server.URL
			bearerToken = server.BearerToken
			devID = server.DevID
			experimentalMode = server.ExperimentalMode
			serverIDForExec = serverIDOverride

			if server.DefaultDatabase != nil {
				connectionString = server.DefaultDatabase.ConnectionString
			}
		}
	} else if req.Server != nil {
		url = req.Server.URL
		bearerToken = req.Server.BearerToken
		devID = req.Server.DevID
		experimentalMode = req.Server.ExperimentalMode
		serverIDForExec = &req.Server.ID

		if req.Server.DefaultDatabase != nil {
			connectionString = req.Server.DefaultDatabase.ConnectionString
		}
	}

	// Apply overrides
	if urlOverride != "" {
		url = urlOverride
	}
	if bearerTokenOverride != "" {
		bearerToken = bearerTokenOverride
	}
	if devIDOverride != "" {
		devID = devIDOverride
	}

	// Determine request data to use
	requestData := req.RequestData
	if requestDataOverride != "" {
		requestData = requestDataOverride
	}

	// Convert requestID to pointer for SampleID
	sampleID := uint(requestID)

	// Create execution record immediately with pending status
	execution := &store.ExecutedRequest{
		SampleID:            &sampleID,
		ServerID:            serverIDForExec,
		RequestIDHeader:     requestIDHeader,
		RequestBody:         requestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0, // 0 indicates pending
		BearerTokenOverride: bearerTokenOverride,
		DevIDOverride:       devIDOverride,
	}

	// Save execution immediately
	execID, err := wa.store.CreateExecution(execution)
	if err != nil {
		slog.Error("failed to save execution", "error", err)
		return 0
	}

	// Execute HTTP request in background
	go func() {
		startTime := time.Now()
		statusCode, responseBody, responseHeaders, err := httputil.MakeHTTPRequest(url, []byte(requestData), requestIDHeader, bearerToken, devID, experimentalMode)
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
					execution.Error = fmt.Sprintf("GraphQL errors in response: %s at %s", message, key)
				}
			}
		}

		// Update execution with results
		execution.ID = uint(execID)
		if err := wa.store.UpdateExecution(execution); err != nil {
			slog.Error("failed to update execution", "error", err)
			return
		}

		slog.Info("request executed", "request_id", requestID, "header_id", requestIDHeader, "status", statusCode, "duration_ms", execution.DurationMS)

		// Collect logs
		collectedLogs := httputil.CollectLogsForRequest(requestIDHeader, wa.logStore, 10*time.Second)

		// Save logs
		if len(collectedLogs) > 0 {
			if err := wa.store.SaveExecutionLogs(execID, collectedLogs); err != nil {
				slog.Error("failed to save logs", "error", err)
			}
		}

		// Extract and save SQL queries
		sqlQueries := sqlutil.ExtractSQLQueries(collectedLogs)
		if len(sqlQueries) > 0 {
			if err := wa.store.SaveSQLQueries(execID, sqlQueries); err != nil {
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
					if err := wa.store.UpdateQueryExplainPlan(execID, q.QueryHash, string(planJSON)); err != nil {
						slog.Error("failed to save EXPLAIN plan", "query_index", i, "error", err)
					}
				}
			}
		}
	}()

	return execID
}

