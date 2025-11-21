package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

// HandleListRequests lists all saved requests
func (c *Controller) HandleListRequests(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	requests, err := c.store.ListRequests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// HandleCreateRequest creates a new request
func (c *Controller) HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var input struct {
		Name        string `json:"name"`
		ServerID    *uint  `json:"serverId,omitempty"`
		URL         string `json:"url,omitempty"`
		BearerToken string `json:"bearerToken,omitempty"`
		DevID       string `json:"devId,omitempty"`
		RequestData string `json:"requestData"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var serverID *uint
	if input.ServerID != nil {
		serverID = input.ServerID
	} else if input.URL != "" {
		server := &store.Server{
			Name:        input.URL,
			URL:         input.URL,
			BearerToken: input.BearerToken,
			DevID:       input.DevID,
		}

		sid, err := c.store.CreateServer(server)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create server: %v", err), http.StatusInternalServerError)
			return
		}
		sidUint := uint(sid)
		serverID = &sidUint
	}

	req := &store.SampleQuery{
		Name:        input.Name,
		ServerID:    serverID,
		RequestData: input.RequestData,
	}

	id, err := c.store.CreateRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

// HandleGetRequest gets a request by ID
func (c *Controller) HandleGetRequest(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	req, err := c.store.GetRequest(id)
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
}

// HandleDeleteRequest deletes a request
func (c *Controller) HandleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	if err := c.store.DeleteRequest(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleExecuteRequest executes a saved request
func (c *Controller) HandleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var input struct {
		ServerID                 *uint  `json:"serverId,omitempty"`
		URLOverride              string `json:"urlOverride,omitempty"`
		BearerTokenOverride      string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride            string `json:"devIdOverride,omitempty"`
		RequestDataOverride      string `json:"requestDataOverride,omitempty"`
		ExperimentalModeOverride string `json:"experimentalModeOverride,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		slog.Warn("failed to parse execute request overrides", "error", err)
	}

	req, err := c.store.GetRequest(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req == nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	var serverID *uint
	var url, bearerToken, devID, experimentalMode string

	if input.ServerID != nil {
		serverID = input.ServerID
	} else if req.ServerID != nil {
		serverID = req.ServerID
	}

	if serverID != nil {
		server, err := c.store.GetServer(int64(*serverID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if server != nil {
			url = server.URL
			bearerToken = server.BearerToken
			devID = server.DevID
			experimentalMode = server.ExperimentalMode
		}
	}

	if input.URLOverride != "" {
		url = input.URLOverride
	}
	if input.BearerTokenOverride != "" {
		bearerToken = input.BearerTokenOverride
	}
	if input.DevIDOverride != "" {
		devID = input.DevIDOverride
	}
	if input.ExperimentalModeOverride != "" {
		experimentalMode = input.ExperimentalModeOverride
	}

	requestData := req.RequestData
	if input.RequestDataOverride != "" {
		requestData = input.RequestDataOverride
	}

	requestIDHeader := httputil.GenerateRequestID()

	exec := &store.ExecutedRequest{
		SampleID:            &req.ID,
		ServerID:            serverID,
		RequestIDHeader:     requestIDHeader,
		RequestBody:         requestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0,
		BearerTokenOverride: input.BearerTokenOverride,
		DevIDOverride:       input.DevIDOverride,
	}

	execID, err := c.store.CreateExecution(exec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		startTime := time.Now()
		statusCode, responseBody, responseHeaders, err := httputil.MakeHTTPRequest(url, []byte(requestData), requestIDHeader, bearerToken, devID, experimentalMode)
		exec.DurationMS = time.Since(startTime).Milliseconds()
		exec.StatusCode = statusCode
		exec.ResponseBody = responseBody
		exec.ResponseHeaders = responseHeaders

		if err != nil {
			exec.Error = err.Error()
		}

		exec.ID = uint(execID)
		if err := c.store.UpdateExecution(exec); err != nil {
			slog.Error("failed to update execution", "error", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "started",
		"executionId": execID,
	})
}
