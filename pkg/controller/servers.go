package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

// HandleServers manages server list and creation
func (c *Controller) HandleServers(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		servers, err := c.store.ListServers()
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
		id, err := c.store.CreateServer(&server)
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

// HandleServerDetail manages individual server operations
func (c *Controller) HandleServerDetail(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid server ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		server, err := c.store.GetServer(id)
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
		if err := c.store.UpdateServer(&server); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := c.store.DeleteServer(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleDatabaseURLs manages database URL list and creation
func (c *Controller) HandleDatabaseURLs(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		dbURLs, err := c.store.ListDatabaseURLs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dbURLs)
	case http.MethodPost:
		var dbURL store.DatabaseURL
		if err := json.NewDecoder(r.Body).Decode(&dbURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := c.store.CreateDatabaseURL(&dbURL)
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

// HandleDatabaseURLDetail manages individual database URL operations
func (c *Controller) HandleDatabaseURLDetail(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid database URL ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		dbURL, err := c.store.GetDatabaseURL(id)
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
		if err := c.store.UpdateDatabaseURL(&dbURL); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := c.store.DeleteDatabaseURL(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
