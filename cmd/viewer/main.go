package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/websocket"
	"github.com/lmittmann/tint"
)

type ClientFilter struct {
	SelectedContainers []string           `json:"selectedContainers"`
	SelectedLevels     []string           `json:"selectedLevels"`
	SearchQuery        string             `json:"searchQuery"`
	TraceFilters       []TraceFilterValue `json:"traceFilters"`
}

type TraceFilterValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Client struct {
	conn   *websocket.Conn
	filter ClientFilter
	mu     sync.RWMutex
}

type WebApp struct {
	docker              *logs.DockerClient
	logStore            *logstore.LogStore // Indexed log storage
	containers          []logs.Container
	containerIDNames    map[string]string // Maps container ID to name
	containerMutex      sync.RWMutex
	clients             map[*Client]bool
	clientsMutex        sync.RWMutex
	logChan             chan logs.LogMessage
	batchChan           chan struct{}
	logBatch            []logs.LogMessage
	batchMutex          sync.Mutex
	ctx                 context.Context
	cancel              context.CancelFunc
	upgrader            websocket.Upgrader
	store               *store.Store
	lastTimestamps      map[string]time.Time // Last timestamp seen per container
	lastTimestampsMutex sync.RWMutex
	shutdownOnce        sync.Once       // Ensure shutdown happens only once
	activeStreams       map[string]bool // Tracks which containers have active log streams
	activeStreamsMutex  sync.RWMutex
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ContainersUpdateMessage struct {
	Containers      []logs.Container         `json:"containers"`
	PortToServerMap map[int]string           `json:"portToServerMap"`
	LogCounts       map[string]int           `json:"logCounts"`  // container name -> log count
	Retentions      map[string]RetentionInfo `json:"retentions"` // container name -> retention settings
}

type RetentionInfo struct {
	Type  string `json:"type"`  // "count" or "time"
	Value int    `json:"value"` // number of logs or seconds
}

type LogWSMessage struct {
	ContainerID string         `json:"containerId"`
	Timestamp   time.Time      `json:"timestamp"`
	Entry       *logs.LogEntry `json:"entry"`
}

// Helper functions to convert between logs.LogMessage and logstore.LogMessage

// serializeLogEntry converts a logs.LogEntry into fields for logstore
func serializeLogEntry(entry *logs.LogEntry) (message string, fields map[string]string) {
	if entry == nil {
		return "", make(map[string]string)
	}

	fields = make(map[string]string)
	message = entry.Message

	// Store the raw log line as a field
	if entry.Raw != "" {
		fields["_raw"] = entry.Raw
	}
	if entry.Timestamp != "" {
		fields["_timestamp"] = entry.Timestamp
	}
	if entry.Level != "" {
		fields["_level"] = entry.Level
	}
	if entry.File != "" {
		fields["_file"] = entry.File
	}

	// Copy all parsed fields
	for k, v := range entry.Fields {
		fields[k] = v
	}

	// Store JSON flag
	if entry.IsJSON {
		fields["_is_json"] = "true"
	}

	return message, fields
}

// deserializeLogEntry reconstructs a logs.LogEntry from logstore.LogMessage
func deserializeLogEntry(msg *logstore.LogMessage) *logs.LogEntry {
	if msg == nil {
		return nil
	}

	entry := &logs.LogEntry{
		Message: msg.Message,
		Fields:  make(map[string]string),
	}

	// Extract special fields
	if raw, ok := msg.Fields["_raw"]; ok {
		entry.Raw = raw
	}
	if timestamp, ok := msg.Fields["_timestamp"]; ok {
		entry.Timestamp = timestamp
	}
	if level, ok := msg.Fields["_level"]; ok {
		entry.Level = level
	}
	if file, ok := msg.Fields["_file"]; ok {
		entry.File = file
	}
	if isJSON, ok := msg.Fields["_is_json"]; ok {
		entry.IsJSON = isJSON == "true"
	}

	// Copy non-special fields
	for k, v := range msg.Fields {
		if !strings.HasPrefix(k, "_") {
			entry.Fields[k] = v
		}
	}

	return entry
}

func NewWebApp() (*WebApp, error) {
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	docker, err := logs.NewDockerClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Open store
	db, err := store.NewStore("graphql-requests.db")
	if err != nil {
		slog.Warn("failed to open database", "error", err)
		db = nil
	}

	app := &WebApp{
		docker:           docker,
		logStore:         logstore.NewLogStore(10000, 2*time.Hour),
		containerIDNames: make(map[string]string),
		clients:          make(map[*Client]bool),
		logChan:          make(chan logs.LogMessage, 1000),
		batchChan:        make(chan struct{}),
		logBatch:         make([]logs.LogMessage, 0, 100),
		ctx:              ctx,
		cancel:           cancel,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		store:          db,
		lastTimestamps: make(map[string]time.Time),
		activeStreams:  make(map[string]bool),
	}

	return app, nil
}

func (wa *WebApp) loadContainerRetentions() error {
	retentionList, err := wa.store.ListContainerRetentions()
	if err != nil {
		return err
	}
	for _, retention := range retentionList {
		for _, container := range wa.containers {
			if container.Name == retention.ContainerName {
				containerID := container.ID
				slog.Info("setting container retention", "containerID", containerID, "retention", retention)
				wa.logStore.SetContainerRetention(containerID, logstore.ContainerRetentionPolicy{
					Type:  retention.RetentionType,
					Value: retention.RetentionValue,
				})
			}
		}
	}
	return nil
}

func (wa *WebApp) broadcastBatch(batch []logs.LogMessage) {
	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	if len(batch) == 0 {
		return
	}

	for client := range wa.clients {
		client.mu.RLock()
		filter := client.filter
		client.mu.RUnlock()

		// Filter logs for this client using matchesFilter
		filteredLogs := []LogWSMessage{}
		for _, msg := range batch {
			if wa.matchesFilter(msg, filter) {
				filteredLogs = append(filteredLogs, LogWSMessage{
					ContainerID: msg.ContainerID,
					Timestamp:   msg.Timestamp,
					Entry:       msg.Entry,
				})
			}
		}

		if len(filteredLogs) == 0 {
			continue
		}

		wsMsg := WSMessage{
			Type: "logs",
		}
		data, _ := json.Marshal(filteredLogs)
		wsMsg.Data = data

		err := client.conn.WriteJSON(wsMsg)
		if err != nil {
			client.conn.Close()
			delete(wa.clients, client)
		}
	}
}

func (wa *WebApp) processLogs() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	logCount := 0
	receivedCount := 0

	slog.Info("processLogs goroutine started")

	for {
		select {
		case <-wa.ctx.Done():
			slog.Info("processLogs goroutine exiting", "totalReceived", receivedCount, "totalProcessed", logCount)
			return
		case msg, ok := <-wa.logChan:
			if !ok {
				// Channel closed, exit
				slog.Info("processLogs goroutine exiting (channel closed)", "totalReceived", receivedCount, "totalProcessed", logCount)
				return
			}
			receivedCount++

			// if receivedCount <= 10 || receivedCount%1000 == 0 {
			// 	slog.Debug("processLogs received message", "receivedCount", receivedCount, "containerID", msg.ContainerID[:12])
			// }

			// Determine the timestamp to use for this log entry
			var logTimestamp time.Time

			// Try to parse the timestamp from the log entry
			if msg.Entry != nil && msg.Entry.Timestamp != "" {
				if parsedTime, ok := logs.ParseTimestamp(msg.Entry.Timestamp); ok {
					logTimestamp = parsedTime

					// Update last timestamp for this container
					wa.lastTimestampsMutex.Lock()
					wa.lastTimestamps[msg.ContainerID] = parsedTime
					wa.lastTimestampsMutex.Unlock()
				}
			}

			// If we couldn't parse a timestamp, check if we have a last timestamp for this container
			if logTimestamp.IsZero() {
				wa.lastTimestampsMutex.RLock()
				lastTS, hasLastTS := wa.lastTimestamps[msg.ContainerID]
				wa.lastTimestampsMutex.RUnlock()

				if hasLastTS {
					// Use the last timestamp for this container (interpolation)
					logTimestamp = lastTS
				} else {
					// No timestamp available, fall back to time.Now()
					logTimestamp = msg.Timestamp
				}
			}

			// Convert logs.LogMessage to logstore.LogMessage and add to store
			message, fields := serializeLogEntry(msg.Entry)
			storeMsg := &logstore.LogMessage{
				Timestamp:   logTimestamp,
				ContainerID: msg.ContainerID,
				Message:     message,
				Fields:      fields,
			}
			wa.logStore.Add(storeMsg)
			logCount++

			// if receivedCount%100 == 0 {
			// 	slog.Debug("processLogs total in memory", "receivedCount", receivedCount, "totalInMemory", wa.logStore.Count())
			// }

			// Add to batch
			wa.batchMutex.Lock()
			wa.logBatch = append(wa.logBatch, msg)
			wa.batchMutex.Unlock()

		case <-ticker.C:
			// Send batch if non-empty
			wa.batchMutex.Lock()
			if len(wa.logBatch) > 0 {
				batch := make([]logs.LogMessage, len(wa.logBatch))
				copy(batch, wa.logBatch)
				wa.logBatch = wa.logBatch[:0]
				wa.batchMutex.Unlock()
				wa.broadcastBatch(batch)
			} else {
				wa.batchMutex.Unlock()
			}
		}
	}
}

func (wa *WebApp) loadContainers() error {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		return err
	}

	wa.containers = containers

	// Update container ID to name mapping
	wa.containerMutex.Lock()
	for _, c := range containers {
		wa.containerIDNames[c.ID] = c.Name
	}
	wa.containerMutex.Unlock()

	for _, c := range containers {
		slog.Info("starting log stream for container", "container_id", c.ID[:12], "container_name", c.Name)
		containerID := c.ID
		onStreamEnd := func() {
			wa.activeStreamsMutex.Lock()
			delete(wa.activeStreams, containerID)
			wa.activeStreamsMutex.Unlock()
			slog.Debug("stream ended, removed from active streams", "container_id", containerID[:12])
		}
		wa.activeStreamsMutex.Lock()
		wa.activeStreams[c.ID] = true
		wa.activeStreamsMutex.Unlock()
		if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan, onStreamEnd); err != nil {
			slog.Error("failed to stream logs", "container_id", c.ID[:12], "container_name", c.Name, "error", err)
			wa.activeStreamsMutex.Lock()
			delete(wa.activeStreams, c.ID)
			wa.activeStreamsMutex.Unlock()
		}
	}

	slog.Info("loaded containers and started log streams", "container_count", len(containers))
	return nil
}

func (wa *WebApp) monitorContainers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	slog.Info("monitorContainers goroutine started")

	previousIDs := make(map[string]bool)
	for _, c := range wa.containers {
		previousIDs[c.ID] = true
	}

	for {
		select {
		case <-wa.ctx.Done():
			slog.Info("monitorContainers goroutine exiting", "containersTracked", len(previousIDs))
			return
		case <-ticker.C:
			containers, err := wa.docker.ListRunningContainers(wa.ctx)
			if err != nil {
				slog.Error("failed to list containers", "error", err)
				continue
			}

			currentIDs := make(map[string]bool)
			for _, c := range containers {
				currentIDs[c.ID] = true
			}

			wa.activeStreamsMutex.RLock()
			activeStreams := make(map[string]bool)
			for id := range wa.activeStreams {
				activeStreams[id] = true
			}
			wa.activeStreamsMutex.RUnlock()

			for _, c := range containers {
				// Check if container is new or if it's running but doesn't have an active stream
				if !previousIDs[c.ID] {
					// New container - start stream
					// Add to container name map
					wa.containerMutex.Lock()
					wa.containerIDNames[c.ID] = c.Name
					wa.containerMutex.Unlock()

					slog.Info("starting log stream for new container", "container_id", c.ID[:12], "container_name", c.Name)
					containerID := c.ID
					onStreamEnd := func() {
						wa.activeStreamsMutex.Lock()
						delete(wa.activeStreams, containerID)
						wa.activeStreamsMutex.Unlock()
						slog.Debug("stream ended, removed from active streams", "container_id", containerID[:12])
					}
					wa.activeStreamsMutex.Lock()
					wa.activeStreams[c.ID] = true
					wa.activeStreamsMutex.Unlock()
					if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan, onStreamEnd); err != nil {
						slog.Error("failed to stream logs for new container", "container_id", c.ID[:12], "container_name", c.Name, "error", err)
						wa.activeStreamsMutex.Lock()
						delete(wa.activeStreams, c.ID)
						wa.activeStreamsMutex.Unlock()
					}
				} else if !activeStreams[c.ID] {
					// Container is running but stream ended (e.g., EOF) - restart it
					// Remove from previousIDs so it will be checked again
					delete(previousIDs, c.ID)

					slog.Info("container stream ended, restarting stream", "container_id", c.ID[:12], "container_name", c.Name)
					containerID := c.ID
					onStreamEnd := func() {
						wa.activeStreamsMutex.Lock()
						delete(wa.activeStreams, containerID)
						wa.activeStreamsMutex.Unlock()
						slog.Info("stream ended, removed from active streams", "container_id", containerID[:12])
					}
					wa.activeStreamsMutex.Lock()
					wa.activeStreams[c.ID] = true
					wa.activeStreamsMutex.Unlock()
					if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan, onStreamEnd); err != nil {
						slog.Error("failed to restart stream for container", "container_id", c.ID[:12], "container_name", c.Name, "error", err)
						wa.activeStreamsMutex.Lock()
						delete(wa.activeStreams, c.ID)
						wa.activeStreamsMutex.Unlock()
					}
				}
			}

			for id := range previousIDs {
				if !currentIDs[id] {
					slog.Info("container stopped", "container_id", id)

					// Remove from container name map
					wa.containerMutex.Lock()
					delete(wa.containerIDNames, id)
					wa.containerMutex.Unlock()

					// Remove from active streams if present
					wa.activeStreamsMutex.Lock()
					delete(wa.activeStreams, id)
					wa.activeStreamsMutex.Unlock()
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

func (wa *WebApp) buildPortToServerMap(containers []logs.Container) map[int]string {
	portToServerMap := make(map[int]string)

	if wa.store == nil {
		return portToServerMap
	}

	// Get all servers with default databases
	servers, err := wa.store.ListServers()
	if err != nil {
		slog.Error("failed to list servers for port mapping", "error", err)
		return portToServerMap
	}

	// Build a map of ports exposed by containers
	for _, container := range containers {
		for _, port := range container.Ports {
			if port.PublicPort > 0 {
				for _, server := range servers {
					if strings.Contains(server.URL, fmt.Sprintf(":%d", port.PublicPort)) {
						if server.DefaultDatabase != nil && server.DefaultDatabase.ConnectionString != "" {
							portToServerMap[port.PublicPort] = server.DefaultDatabase.ConnectionString
							slog.Debug("mapped container port to server", "port", port, "server", server.Name, "container", container.Name)
						}
						break
					}
				}
			}
		}
	}

	return portToServerMap
}

func (wa *WebApp) broadcastContainerUpdate(containers []logs.Container) {
	portToServerMap := wa.buildPortToServerMap(containers)

	// Get log counts for each container
	logCounts := make(map[string]int)
	for _, container := range containers {
		count := wa.logStore.CountByContainer(container.ID)
		logCounts[container.Name] = count
	}

	// Get retention settings for all containers
	retentions := make(map[string]RetentionInfo)
	if wa.store != nil {
		retentionList, err := wa.store.ListContainerRetentions()
		if err == nil {
			for _, r := range retentionList {
				retentions[r.ContainerName] = RetentionInfo{
					Type:  r.RetentionType,
					Value: r.RetentionValue,
				}
			}
		}
	}

	update := ContainersUpdateMessage{
		Containers:      containers,
		PortToServerMap: portToServerMap,
		LogCounts:       logCounts,
		Retentions:      retentions,
	}

	wsMsg := WSMessage{
		Type: "containers",
	}
	data, _ := json.Marshal(update)
	wsMsg.Data = data

	wa.clientsMutex.Lock()
	defer wa.clientsMutex.Unlock()

	for client := range wa.clients {
		err := client.conn.WriteJSON(wsMsg)
		if err != nil {
			client.conn.Close()
			delete(wa.clients, client)
		}
	}
}


func (wa *WebApp) Run(addr string) error {
	if err := wa.loadContainers(); err != nil {
		return err
	}

	// Try to initialize database connection for EXPLAIN queries
	if err := sqlexplain.Init(); err != nil {
		slog.Warn("DATABASE_URL not set, database connection not available (EXPLAIN feature disabled)", "error", err)
	} else {
		slog.Info("database connection established for EXPLAIN queries")
	}

	slog.Info("starting background goroutines")
	go wa.processLogs()
	go wa.monitorContainers()

	if err := wa.loadContainerRetentions(); err != nil {
		slog.Error("failed to load container retentions", "error", err)
	} else {
		slog.Info("loaded container retentions")
	}

	http.HandleFunc("/api/containers", loggingMiddleware(wa.handleContainers))
	http.HandleFunc("/api/logs", loggingMiddleware(wa.handleLogs))
	http.HandleFunc("/api/ws", loggingMiddleware(wa.handleWebSocket))
	http.HandleFunc("/api/explain", loggingMiddleware(wa.handleExplain))
	http.HandleFunc("/api/save-trace", loggingMiddleware(wa.handleSaveTrace))
	http.HandleFunc("/api/execute", loggingMiddleware(wa.handleExecute))
	http.HandleFunc("/debug", loggingMiddleware(wa.handleDebug))

	// Request management endpoints
	http.HandleFunc("/api/servers", loggingMiddleware(wa.handleServers))
	http.HandleFunc("/api/servers/", loggingMiddleware(wa.handleServerDetail))
	http.HandleFunc("/api/database-urls", loggingMiddleware(wa.handleDatabaseURLs))
	http.HandleFunc("/api/database-urls/", loggingMiddleware(wa.handleDatabaseURLDetail))
	http.HandleFunc("/api/requests", loggingMiddleware(wa.handleRequests))
	http.HandleFunc("/api/requests/", loggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/execute") {
			wa.handleExecuteRequest(w, r)
		} else {
			wa.handleRequestDetail(w, r)
		}
	}))
	http.HandleFunc("/api/executions", loggingMiddleware(wa.handleExecutions))
	http.HandleFunc("/api/all-executions", loggingMiddleware(wa.handleAllExecutions))
	http.HandleFunc("/api/executions/", loggingMiddleware(wa.handleExecutionDetail))
	http.HandleFunc("/api/sql/", loggingMiddleware(wa.handleSQLDetail))
	http.HandleFunc("/api/retention", loggingMiddleware(wa.handleRetention))
	http.HandleFunc("/api/retention/", loggingMiddleware(wa.handleRetentionDetail))

	// Serve static assets from Vite build output
	// In production, serve from dist folder built by Vite
	// In development, the Vite dev server will handle this
	distDir := "./web/dist"
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		slog.Warn("dist directory not found. Run 'npm run build' in web directory.")
		distDir = "./web"
	}

	// Serve static files with correct MIME types
	serveFiles := func(prefix, dir string) {
		fs := http.FileServer(http.Dir(dir))
		http.Handle(prefix, http.StripPrefix(prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Let http.FileServer handle MIME type detection
			fs.ServeHTTP(w, r)
		})))
	}

	serveFiles("/static/", distDir+"/static")
	serveFiles("/assets/", distDir+"/assets")
	serveFiles("/js/", distDir+"/js")
	serveFiles("/css/", distDir+"/css")
	serveFiles("/lib/", distDir+"/lib")

	// Serve SPA for all non-API and non-static routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If it's an API route, let it 404 naturally
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Serve static files (handled by above handlers)
		if strings.HasPrefix(r.URL.Path, "/static/") ||
			strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasPrefix(r.URL.Path, "/js/") ||
			strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/lib/") {
			http.NotFound(w, r)
			return
		}

		// For SPA routes, serve index.html from dist folder for all other paths
		// This enables Vue Router client-side routing
		indexPath := filepath.Join(distDir, "index.html")
		http.ServeFile(w, r, indexPath)
	})

	// Create HTTP server with graceful shutdown
	server := &http.Server{
		Addr:    addr,
		Handler: nil,
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		slog.Info("server starting", "address", "http://localhost"+addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for interrupt signal or server error
	select {
	case err := <-errChan:
		slog.Error("server error, shutting down", "error", err)
		wa.shutdown()
		return err
	case sig := <-sigChan:
		slog.Info("received shutdown signal", "signal", sig)

		// Shutdown gracefully
		wa.shutdown()

		// Give goroutines a moment to exit
		time.Sleep(200 * time.Millisecond)

		// Create shutdown context with timeout
		slog.Info("initiating graceful server shutdown", "timeout", "5s")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		// Shutdown server gracefully
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
			return err
		}

		slog.Info("server shutdown complete")
		return nil
	}
}

func main() {
	slog.Info("application starting")

	app, err := NewWebApp()
	if err != nil {
		slog.Error("failed to create app", "error", err)
		os.Exit(1)
	}

	defer func() {
		slog.Info("application shutting down, cleaning up resources")
		// Shutdown will handle context cancellation
		app.shutdown()

		slog.Info("closing SQL explain connection")
		sqlexplain.Close()

		if app.store != nil {
			slog.Info("closing database store")
			app.store.Close()
		}

		if app.docker != nil {
			slog.Info("closing Docker client")
			app.docker.Close()
		}

		slog.Info("cleanup complete")
	}()

	if err := app.Run(":9000"); err != nil {
		slog.Error("failed to run server", "error", err)
		os.Exit(1)
	}
}
