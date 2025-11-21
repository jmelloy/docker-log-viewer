package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

// HandleListServers lists all servers
func (c *Controller) HandleListServers(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	servers, err := c.store.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// HandleCreateServer creates a new server
func (c *Controller) HandleCreateServer(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

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
}

// HandleGetServer gets a server by ID
func (c *Controller) HandleGetServer(w http.ResponseWriter, r *http.Request) {
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
}

// HandleUpdateServer updates a server
func (c *Controller) HandleUpdateServer(w http.ResponseWriter, r *http.Request) {
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
}

// HandleDeleteServer deletes a server
func (c *Controller) HandleDeleteServer(w http.ResponseWriter, r *http.Request) {
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

	if err := c.store.DeleteServer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleListDatabaseURLs lists all database URLs
func (c *Controller) HandleListDatabaseURLs(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	dbURLs, err := c.store.ListDatabaseURLs()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dbURLs)
}

// HandleCreateDatabaseURL creates a new database URL
func (c *Controller) HandleCreateDatabaseURL(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

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
}

// HandleGetDatabaseURL gets a database URL by ID
func (c *Controller) HandleGetDatabaseURL(w http.ResponseWriter, r *http.Request) {
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
}

// HandleUpdateDatabaseURL updates a database URL
func (c *Controller) HandleUpdateDatabaseURL(w http.ResponseWriter, r *http.Request) {
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
}

// HandleDeleteDatabaseURL deletes a database URL
func (c *Controller) HandleDeleteDatabaseURL(w http.ResponseWriter, r *http.Request) {
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

	if err := c.store.DeleteDatabaseURL(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
