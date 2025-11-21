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
	r.HandleFunc("/api/retention", c.HandleRetention).Methods("GET", "POST")
	r.HandleFunc("/api/retention/{containerName}", c.HandleRetentionDetail).Methods("GET", "DELETE")

	// Server management endpoints
	r.HandleFunc("/api/servers", c.HandleServers).Methods("GET", "POST")
	r.HandleFunc("/api/servers/{id}", c.HandleServerDetail).Methods("GET", "PUT", "DELETE")

	// Database URL endpoints
	r.HandleFunc("/api/database-urls", c.HandleDatabaseURLs).Methods("GET", "POST")
	r.HandleFunc("/api/database-urls/{id}", c.HandleDatabaseURLDetail).Methods("GET", "PUT", "DELETE")

	// Request management endpoints (these need to be added to separate files)
	// r.HandleFunc("/api/requests", c.HandleRequests).Methods("GET", "POST")
	// r.HandleFunc("/api/requests/{id}", c.HandleRequestDetail).Methods("GET", "PUT", "DELETE")
	// r.HandleFunc("/api/requests/{id}/execute", c.HandleExecuteRequest).Methods("POST")

	// Execution endpoints
	// r.HandleFunc("/api/executions", c.HandleExecutions).Methods("GET")
	// r.HandleFunc("/api/all-executions", c.HandleAllExecutions).Methods("GET")
	// r.HandleFunc("/api/executions/{id}", c.HandleExecutionDetail).Methods("GET")
	// r.HandleFunc("/api/executions/{id}/export-notion", c.HandleExecutionNotionExport).Methods("POST")

	// SQL endpoints
	// r.HandleFunc("/api/sql/{hash}", c.HandleSQLDetail).Methods("GET")
	// r.HandleFunc("/api/sql/{hash}/export-notion", c.HandleSQLNotionExport).Methods("POST")
	// r.HandleFunc("/api/explain", c.HandleExplain).Methods("POST")
	// r.HandleFunc("/api/save-trace", c.HandleSaveTrace).Methods("POST")
	// r.HandleFunc("/api/execute", c.HandleExecute).Methods("POST")

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
