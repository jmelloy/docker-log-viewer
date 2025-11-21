package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// SetupRouter creates and configures the Gorilla mux router
func (c *Controller) SetupRouter() *mux.Router {
	r := mux.NewRouter()

	// Apply logging middleware to all routes
	r.Use(loggingMiddleware)

	// Container and log endpoints
	r.HandleFunc("/api/containers", c.HandleContainers).Methods("GET")
	r.HandleFunc("/api/logs", c.HandleLogs).Methods("GET")
	r.HandleFunc("/api/ws", c.HandleWebSocket).Methods("GET")
	r.HandleFunc("/debug", c.HandleDebug).Methods("GET")

	// Retention endpoints
	r.HandleFunc("/api/retention", c.HandleListRetentions).Methods("GET")
	r.HandleFunc("/api/retention", c.HandleCreateRetention).Methods("POST")
	r.HandleFunc("/api/retention/{containerName}", c.HandleGetRetention).Methods("GET")
	r.HandleFunc("/api/retention/{containerName}", c.HandleDeleteRetention).Methods("DELETE")

	// Server management endpoints
	r.HandleFunc("/api/servers", c.HandleListServers).Methods("GET")
	r.HandleFunc("/api/servers", c.HandleCreateServer).Methods("POST")
	r.HandleFunc("/api/servers/{id}", c.HandleGetServer).Methods("GET")
	r.HandleFunc("/api/servers/{id}", c.HandleUpdateServer).Methods("PUT")
	r.HandleFunc("/api/servers/{id}", c.HandleDeleteServer).Methods("DELETE")

	// Database URL endpoints
	r.HandleFunc("/api/database-urls", c.HandleListDatabaseURLs).Methods("GET")
	r.HandleFunc("/api/database-urls", c.HandleCreateDatabaseURL).Methods("POST")
	r.HandleFunc("/api/database-urls/{id}", c.HandleGetDatabaseURL).Methods("GET")
	r.HandleFunc("/api/database-urls/{id}", c.HandleUpdateDatabaseURL).Methods("PUT")
	r.HandleFunc("/api/database-urls/{id}", c.HandleDeleteDatabaseURL).Methods("DELETE")

	return r
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		next.ServeHTTP(w, r)

		slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"duration_ms", time.Since(startTime).Milliseconds(),
		)
	})
}
