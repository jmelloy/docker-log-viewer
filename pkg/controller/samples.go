package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

// HandleListSampleQueries lists all saved requests
func (c *Controller) HandleListSampleQueries(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	requests, err := c.store.ListSampleQueries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// HandleCreateSampleQuery creates a new request
func (c *Controller) HandleCreateSampleQuery(w http.ResponseWriter, r *http.Request) {
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

	id, err := c.store.CreateSampleQuery(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

// HandleGetSampleQuery gets a request by ID
func (c *Controller) HandleGetSampleQuery(w http.ResponseWriter, r *http.Request) {
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

	req, err := c.store.GetSampleQuery(id)
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

// HandleDeleteSampleQuery deletes a request
func (c *Controller) HandleDeleteSampleQuery(w http.ResponseWriter, r *http.Request) {
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

	if err := c.store.DeleteSampleQuery(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
