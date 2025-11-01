package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"
	"docker-log-parser/pkg/utils"

	"github.com/gorilla/websocket"
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
	docker           *logs.DockerClient
	logStore         *logstore.LogStore // Indexed log storage
	containers       []logs.Container
	containerIDNames map[string]string // Maps container ID to name
	containerMutex   sync.RWMutex
	clients          map[*Client]bool
	clientsMutex     sync.RWMutex
	logChan          chan logs.LogMessage
	batchChan        chan struct{}
	logBatch         []logs.LogMessage
	batchMutex       sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	upgrader         websocket.Upgrader
	store            *store.Store
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ContainersUpdateMessage struct {
	Containers      []logs.Container      `json:"containers"`
	PortToServerMap map[int]string        `json:"portToServerMap"`
	LogCounts       map[string]int        `json:"logCounts"`   // container name -> log count
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
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
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
		store: db,
	}

	return app, nil
}

func (wa *WebApp) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	response := ContainersUpdateMessage{
		Containers:      containers,
		PortToServerMap: portToServerMap,
		LogCounts:       logCounts,
		Retentions:      retentions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (wa *WebApp) handleLogs(w http.ResponseWriter, r *http.Request) {
	// Get recent logs from the store (limit to 1000)
	recentLogs := wa.logStore.GetRecent(1000)

	logs := make([]LogWSMessage, 0, len(recentLogs))
	for _, logMsg := range recentLogs {
		logs = append(logs, LogWSMessage{
			ContainerID: logMsg.ContainerID,
			Timestamp:   logMsg.Timestamp,
			Entry:       deserializeLogEntry(logMsg),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (wa *WebApp) handleDebug(w http.ResponseWriter, r *http.Request) {
	totalLogs := wa.logStore.Count()

	// Count logs by container
	logsByContainer := make(map[string]int)

	wa.containerMutex.RLock()
	containers := make([]map[string]string, 0)
	for id, name := range wa.containerIDNames {
		shortID := id
		if len(id) > 12 {
			shortID = id[:12]
		}
		// Get count from logstore for this container
		count := wa.logStore.CountByContainer(id)
		logsByContainer[id] = count

		containers = append(containers, map[string]string{
			"id":    shortID,
			"name":  name,
			"count": fmt.Sprintf("%d", count),
		})
	}
	wa.containerMutex.RUnlock()

	wa.clientsMutex.RLock()
	clientCount := len(wa.clients)
	clientFilters := make([]map[string]interface{}, 0)
	for client := range wa.clients {
		clientFilters = append(clientFilters, map[string]interface{}{
			"selectedContainers": client.filter.SelectedContainers,
			"selectedLevels":     client.filter.SelectedLevels,
			"searchQuery":        client.filter.SearchQuery,
			"traceFilterCount":   len(client.filter.TraceFilters),
		})
	}
	wa.clientsMutex.RUnlock()

	debugInfo := map[string]interface{}{
		"totalLogsInMemory": totalLogs,
		"containerCount":    len(containers),
		"containers":        containers,
		"connectedClients":  clientCount,
		"clientFilters":     clientFilters,
		"logChannelSize":    len(wa.logChan),
		"logChannelCap":     cap(wa.logChan),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

func (wa *WebApp) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wa.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error", "error", err)
		return
	}

	client := &Client{
		conn: conn,
		filter: ClientFilter{
			SelectedContainers: []string{},
			SelectedLevels:     []string{},
			SearchQuery:        "",
			TraceFilters:       []TraceFilterValue{},
		},
	}

	wa.clientsMutex.Lock()
	wa.clients[client] = true
	wa.clientsMutex.Unlock()

	defer func() {
		wa.clientsMutex.Lock()
		delete(wa.clients, client)
		wa.clientsMutex.Unlock()
		conn.Close()
	}()

	// Read filter updates from client
	for {
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}

		if msg.Type == "filter" {
			var filter ClientFilter
			if err := json.Unmarshal(msg.Data, &filter); err != nil {
				slog.Error("failed to parse filter", "error", err)
				continue
			}
			client.mu.Lock()
			client.filter = filter
			client.mu.Unlock()

			// Send initial filtered logs to the client
			go wa.sendInitialLogs(client)
		}
	}
}

func (wa *WebApp) sendInitialLogs(client *Client) {
	client.mu.RLock()
	filter := client.filter
	client.mu.RUnlock()

	// Get logs from selected containers or all containers if none selected
	var recentStoreLogs []*logstore.LogMessage
	
	if len(filter.SelectedContainers) > 0 {
		// Get logs from specific containers
		wa.containerMutex.RLock()
		for containerID, containerName := range wa.containerIDNames {
			// Check if this container is selected
			for _, selectedName := range filter.SelectedContainers {
				if containerName == selectedName {
					// Get logs for this specific container (up to 10000 per container)
					containerLogs := wa.logStore.SearchByContainer(containerID, 10000)
					recentStoreLogs = append(recentStoreLogs, containerLogs...)
					break
				}
			}
		}
		wa.containerMutex.RUnlock()
	} else {
		// No containers selected, get recent logs globally
		recentStoreLogs = wa.logStore.GetRecent(10000)
	}

	// Convert logstore messages back to logs.LogMessage for filtering
	allLogs := make([]logs.LogMessage, 0, len(recentStoreLogs))
	for _, storeMsg := range recentStoreLogs {
		allLogs = append(allLogs, logs.LogMessage{
			ContainerID: storeMsg.ContainerID,
			Timestamp:   storeMsg.Timestamp,
			Entry:       deserializeLogEntry(storeMsg),
		})
	}

	// Filter logs for this client (apply level, search, trace filters)
	// Skip container filter since we already fetched from selected containers
	filteredLogs := []LogWSMessage{}

	for _, msg := range allLogs {
		if wa.matchesFilterExceptContainer(msg, filter) {
			filteredLogs = append(filteredLogs, LogWSMessage{
				ContainerID: msg.ContainerID,
				Timestamp:   msg.Timestamp,
				Entry:       msg.Entry,
			})
		}
	}

	// Reverse the logs so oldest is first (logs come from store newest-first)
	for i, j := 0, len(filteredLogs)-1; i < j; i, j = i+1, j-1 {
		filteredLogs[i], filteredLogs[j] = filteredLogs[j], filteredLogs[i]
	}

	// Send clear message to replace all logs
	wsMsg := WSMessage{
		Type: "logs_initial",
	}
	data, _ := json.Marshal(filteredLogs)
	wsMsg.Data = data

	client.conn.WriteJSON(wsMsg)
}

// matchesFilterExceptContainer checks if a log matches the filter criteria except container
func (wa *WebApp) matchesFilterExceptContainer(msg logs.LogMessage, filter ClientFilter) bool {
	// Skip container filter - already applied during fetch

	// Level filter
	if len(filter.SelectedLevels) > 0 {
		if msg.Entry == nil {
			return false
		}

		// Check if log has a level
		if msg.Entry.Level == "" {
			// No level parsed - check if NONE is selected
			found := false
			for _, level := range filter.SelectedLevels {
				if level == "NONE" {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else {
			// Has a level - check if it matches (case-insensitive)
			found := false
			logLevel := strings.ToUpper(msg.Entry.Level)
			for _, level := range filter.SelectedLevels {
				if logLevel == level {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Search query filter - AND multiple terms together
	if filter.SearchQuery != "" {
		terms := strings.Fields(filter.SearchQuery) // Split on whitespace

		if msg.Entry != nil {
			for _, term := range terms {
				query := strings.ToLower(term)
				found := false

				// Search in message
				if strings.Contains(strings.ToLower(msg.Entry.Message), query) {
					found = true
				}

				// Search in raw log
				if !found && strings.Contains(strings.ToLower(msg.Entry.Raw), query) {
					found = true
				}

				// Search in fields
				if !found && msg.Entry.Fields != nil {
					for key, value := range msg.Entry.Fields {
						if strings.Contains(strings.ToLower(key), query) || strings.Contains(strings.ToLower(value), query) {
							found = true
							break
						}
					}
				}

				// If any term is not found, the log doesn't match (AND logic)
				if !found {
					return false
				}
			}
		} else {
			// No entry, can't match
			return false
		}
	}

	// Trace filters - all must match
	if len(filter.TraceFilters) > 0 && msg.Entry != nil && msg.Entry.Fields != nil {
		for _, tf := range filter.TraceFilters {
			if val, ok := msg.Entry.Fields[tf.Type]; !ok || val != tf.Value {
				return false
			}
		}
	}

	return true
}

// matchesFilter checks if a log matches the client's filter criteria (including container filter)
func (wa *WebApp) matchesFilter(msg logs.LogMessage, filter ClientFilter) bool {
	// Container filter
	if len(filter.SelectedContainers) > 0 {
		wa.containerMutex.RLock()
		containerName := wa.containerIDNames[msg.ContainerID]
		wa.containerMutex.RUnlock()

		found := false
		for _, name := range filter.SelectedContainers {
			if containerName == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Apply remaining filters
	return wa.matchesFilterExceptContainer(msg, filter)
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

		// Filter logs for this client
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

	slog.Debug("processLogs goroutine started")

	for {
		select {
		case <-wa.ctx.Done():
			slog.Debug("processLogs goroutine exiting")
			return
		case msg := <-wa.logChan:
			receivedCount++

			if receivedCount <= 10 || receivedCount%100 == 0 {
				slog.Debug("processLogs received message", "receivedCount", receivedCount, "containerID", msg.ContainerID[:12])
			}

			// Convert logs.LogMessage to logstore.LogMessage and add to store
			message, fields := serializeLogEntry(msg.Entry)
			storeMsg := &logstore.LogMessage{
				Timestamp:   msg.Timestamp,
				ContainerID: msg.ContainerID,
				Message:     message,
				Fields:      fields,
			}
			wa.logStore.Add(storeMsg)
			logCount++

			if receivedCount%100 == 0 {
				slog.Debug("processLogs total in memory", "receivedCount", receivedCount, "totalInMemory", wa.logStore.Count())
			}

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
		if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
			slog.Error("failed to stream logs", "container_id", c.ID, "error", err)
		}
	}

	return nil
}

func (wa *WebApp) monitorContainers() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	previousIDs := make(map[string]bool)
	for _, c := range wa.containers {
		previousIDs[c.ID] = true
	}

	for {
		select {
		case <-wa.ctx.Done():
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

			for _, c := range containers {
				if !previousIDs[c.ID] {

					// Add to container name map
					wa.containerMutex.Lock()
					wa.containerIDNames[c.ID] = c.Name
					wa.containerMutex.Unlock()

					if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
						slog.Error("failed to stream logs for new container", "container_id", c.ID, "error", err)
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
							slog.Info("mapped container port to server", "port", port, "server", server.Name, "container", container.Name)
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
	update := ContainersUpdateMessage{
		Containers:      containers,
		PortToServerMap: wa.buildPortToServerMap(containers),
	}

	wsMsg := WSMessage{
		Type: "containers",
	}
	data, _ := json.Marshal(update)
	wsMsg.Data = data

	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	for client := range wa.clients {
		err := client.conn.WriteJSON(wsMsg)
		if err != nil {
			client.conn.Close()
			delete(wa.clients, client)
		}
	}
}

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
		TraceID    string                   `json:"traceId"`
		RequestID  string                   `json:"requestId"`
		Name       string                   `json:"name"`
		Logs       []logs.LogMessage        `json:"logs"`
		SQLQueries []map[string]interface{} `json:"sqlQueries"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create an execution entry with the trace data
	requestIDHeader := input.RequestID
	if requestIDHeader == "" {
		requestIDHeader = input.TraceID
	}
	if requestIDHeader == "" {
		requestIDHeader = input.Name
	}

	// Calculate duration from logs
	var durationMS int64
	if len(input.Logs) > 1 {
		firstTime := input.Logs[0].Timestamp
		lastTime := input.Logs[len(input.Logs)-1].Timestamp
		if !firstTime.IsZero() && !lastTime.IsZero() {
			durationMS = lastTime.Sub(firstTime).Milliseconds()
		}
	}

	exec := &store.ExecutedRequest{
		RequestIDHeader: requestIDHeader,
		RequestBody:     fmt.Sprintf(`{"name":"%s","traceId":"%s","requestId":"%s"}`, input.Name, input.TraceID, input.RequestID),
		StatusCode:      200,
		DurationMS:      durationMS,
		ExecutedAt:      time.Now(),
	}

	id, err := wa.store.CreateExecution(exec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save logs for this execution
	if len(input.Logs) > 0 {
		if err := wa.store.SaveExecutionLogs(id, input.Logs); err != nil {
			slog.Error("failed to save execution logs", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"message": "Trace saved successfully as execution",
	})
}

// Request management handlers
func (wa *WebApp) handleRequests(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		wa.listRequests(w, r)
	case http.MethodPost:
		wa.createRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) listRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := wa.store.ListRequests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (wa *WebApp) createRequest(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming request which may have server fields or a serverID
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

	// If URL is provided but no serverID, create a new server
	var serverID *uint
	if input.ServerID != nil {
		// Use existing server
		serverID = input.ServerID
	} else if input.URL != "" {
		// Create new server with URL and credentials
		server := &store.Server{
			Name:        input.URL, // Use URL as name for now
			URL:         input.URL,
			BearerToken: input.BearerToken,
			DevID:       input.DevID,
		}

		sid, err := wa.store.CreateServer(server)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create server: %v", err), http.StatusInternalServerError)
			return
		}
		sidUint := uint(sid)
		serverID = &sidUint
	}

	// Create the request
	req := &store.SampleQuery{
		Name:        input.Name,
		ServerID:    serverID,
		RequestData: input.RequestData,
	}

	id, err := wa.store.CreateRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (wa *WebApp) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		req, err := wa.store.GetRequest(id)
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
	case http.MethodDelete:
		if err := wa.store.DeleteRequest(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	path = strings.TrimSuffix(path, "/execute")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	// Parse request body for overrides
	var input struct {
		ServerID            *uint  `json:"serverId,omitempty"`
		URLOverride         string `json:"urlOverride,omitempty"`
		BearerTokenOverride string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride       string `json:"devIdOverride,omitempty"`
		RequestDataOverride string `json:"requestDataOverride,omitempty"`
	}

	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			// Ignore decode errors for backward compatibility
			slog.Warn("failed to decode execute request body", "error", err)
		}
	}

	// Execute request in background with overrides
	executionID := wa.executeRequestWithOverrides(id, input.ServerID, input.URLOverride, input.BearerTokenOverride, input.DevIDOverride, input.RequestDataOverride)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "started",
		"executionId": executionID,
	})
}

func (wa *WebApp) handleExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract request ID from query param
	requestIDStr := r.URL.Query().Get("request_id")
	if requestIDStr == "" {
		http.Error(w, "request_id parameter required", http.StatusBadRequest)
		return
	}

	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request_id", http.StatusBadRequest)
		return
	}

	executions, err := wa.store.ListExecutions(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (wa *WebApp) handleAllExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	executions, err := wa.store.ListAllExecutions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (wa *WebApp) handleExecutionDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	detail, err := wa.store.GetExecutionDetail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

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
	requestIDHeader := generateRequestID()

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
		SampleID:        &sampleID,
		ServerID:        serverIDForExec,
		RequestIDHeader: requestIDHeader,
		RequestBody:     requestData,
		ExecutedAt:      time.Now(),
		StatusCode:      0, // 0 indicates pending
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
		statusCode, responseBody, responseHeaders, err := makeHTTPRequest(url, []byte(requestData), requestIDHeader, bearerToken, devID, experimentalMode)
		execution.DurationMS = time.Since(startTime).Milliseconds()
		execution.StatusCode = statusCode
		execution.ResponseBody = responseBody
		execution.ResponseHeaders = responseHeaders

		if err != nil {
			execution.Error = err.Error()
		}

		// Update execution with results
		execution.ID = uint(execID)
		if err := wa.store.UpdateExecution(execution); err != nil {
			slog.Error("failed to update execution", "error", err)
			return
		}

		slog.Info("request executed", "request_id", requestID, "header_id", requestIDHeader, "status", statusCode, "duration_ms", execution.DurationMS)

		// Collect logs
		collectedLogs := wa.collectLogsForRequest(requestIDHeader, 10*time.Second)

		// Save logs
		if len(collectedLogs) > 0 {
			if err := wa.store.SaveExecutionLogs(execID, collectedLogs); err != nil {
				slog.Error("failed to save logs", "error", err)
			}
		}

		// Extract and save SQL queries
		sqlQueries := extractSQLQueries(collectedLogs)
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

func (wa *WebApp) executeRequest(requestID int64) {
	wa.executeRequestWithOverrides(requestID, nil, "", "", "", "")
}

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func makeHTTPRequest(url string, data []byte, requestID, bearerToken, devID, experimentalMode string) (int, string, string, error) {
	// Replace localhost with host.docker.internal if running in Docker
	url = utils.ReplaceLocalhostWithDockerHost(url)

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return 0, "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", requestID)

	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if devID != "" {
		req.Header.Set("X-GlueDev-UserID", devID)
	}
	if experimentalMode != "" {
		req.Header.Set("x-glue-experimental-mode", experimentalMode)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	// Capture response headers as JSON
	headersJSON, _ := json.Marshal(resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", string(headersJSON), err
	}

	return resp.StatusCode, string(bodyBytes), string(headersJSON), nil
}

func (wa *WebApp) collectLogsForRequest(requestID string, timeout time.Duration) []logs.LogMessage {
	// Wait for logs to arrive
	time.Sleep(timeout)

	// Search LogStore for matching request ID
	filters := []logstore.FieldFilter{
		{Name: "request_id", Value: requestID},
	}
	storeResults := wa.logStore.SearchByFields(filters, 100000)

	// Convert back to logs.LogMessage
	collected := make([]logs.LogMessage, 0, len(storeResults))
	for _, storeMsg := range storeResults {
		collected = append(collected, logs.LogMessage{
			ContainerID: storeMsg.ContainerID,
			Timestamp:   storeMsg.Timestamp,
			Entry:       deserializeLogEntry(storeMsg),
		})
	}

	return collected
}

func matchesRequestID(msg logs.LogMessage, requestID string) bool {
	if msg.Entry == nil || msg.Entry.Fields == nil {
		return false
	}

	for _, field := range []string{"request_id", "requestId", "requestID", "req_id"} {
		if val, ok := msg.Entry.Fields[field]; ok && val == requestID {
			return true
		}
	}

	return false
}

func extractSQLQueries(logMessages []logs.LogMessage) []store.SQLQuery {
	queries := []store.SQLQuery{}

	for _, msg := range logMessages {
		if msg.Entry == nil || msg.Entry.Message == "" {
			continue
		}

		message := msg.Entry.Message
		// Check for [sql] or [query] format
		if strings.Contains(message, "[sql]") || (msg.Entry.Fields != nil && msg.Entry.Fields["type"] == "query") {
			var sqlText string
			var normalizedQuery string
			var query store.SQLQuery

			// Handle [sql] format
			if strings.Contains(message, "[sql]") {
				sqlMatch := regexp.MustCompile(`\[sql\]:\s*(.+)`).FindStringSubmatch(message)
				if len(sqlMatch) > 1 {
					sqlText = sqlMatch[1]
					normalizedQuery = normalizeQuery(sqlText)
					query = store.SQLQuery{
						Query:           sqlText,
						NormalizedQuery: normalizedQuery,
						QueryHash:       store.ComputeQueryHash(normalizedQuery),
					}
				} else {
					continue
				}
			} else if msg.Entry.Fields != nil && msg.Entry.Fields["type"] == "query" {
				// Handle [query] format - message is the SQL query
				sqlText = message
				normalizedQuery = normalizeQuery(sqlText)
				query = store.SQLQuery{
					Query:           sqlText,
					NormalizedQuery: normalizedQuery,
					QueryHash:       store.ComputeQueryHash(normalizedQuery),
				}

				// Extract duration and rows from fields
				if duration, ok := msg.Entry.Fields["duration_ms"]; ok {
					if durationVal, err := strconv.ParseFloat(duration, 64); err == nil {
						query.DurationMS = durationVal
					}
				}
				if rows, ok := msg.Entry.Fields["rows"]; ok {
					if rowsVal, err := strconv.Atoi(rows); err == nil {
						query.Rows = rowsVal
					}
				}
			} else {
				continue
			}

			if msg.Entry.Fields != nil {
				// These apply to both [sql] and [query] formats
				if duration, ok := msg.Entry.Fields["duration"]; ok {
					var durationVal float64
					if _, err := strconv.ParseFloat(duration, 64); err == nil {
						durationVal, _ = strconv.ParseFloat(duration, 64)
						query.DurationMS = durationVal
					}
				}
				if table, ok := msg.Entry.Fields["db.table"]; ok {
					query.QueriedTable = table
				}
				if op, ok := msg.Entry.Fields["db.operation"]; ok {
					query.Operation = op
				}
				if rows, ok := msg.Entry.Fields["db.rows"]; ok {
					var rowsVal int
					if _, err := strconv.Atoi(rows); err == nil {
						rowsVal, _ = strconv.Atoi(rows)
						query.Rows = rowsVal
					}
				}
				// Store db.vars as JSON for later use in EXPLAIN
				if vars, ok := msg.Entry.Fields["db.vars"]; ok {
					query.Variables = vars
				}
				// Check both gql.operation and gql.operationName for GraphQL operation
				if gqlOp, ok := msg.Entry.Fields["gql.operation"]; ok {
					query.GraphQLOperation = gqlOp
				} else if gqlOp, ok := msg.Entry.Fields["gql.operationName"]; ok {
					query.GraphQLOperation = gqlOp
				}
			}

			queries = append(queries, query)
		}
	}

	return queries
}

func normalizeQuery(query string) string {
	// Replace numbers with ?
	normalized := regexp.MustCompile(`\b\d+\b`).ReplaceAllString(query, "?")
	// Replace $1, $2, etc. with ?
	normalized = regexp.MustCompile(`\$\d+`).ReplaceAllString(normalized, "?")
	// Replace quoted strings with ?
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "?")
	// Collapse whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func (wa *WebApp) handleRetention(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retentions, err := wa.store.ListContainerRetentions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retentions)
	case http.MethodPost:
		var retention store.ContainerRetention
		if err := json.NewDecoder(r.Body).Decode(&retention); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := wa.store.SaveContainerRetention(&retention); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retention)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleRetentionDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract container name from path
	containerName := strings.TrimPrefix(r.URL.Path, "/api/retention/")
	if containerName == "" {
		http.Error(w, "Container name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retention, err := wa.store.GetContainerRetention(containerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if retention == nil {
			http.Error(w, "Retention settings not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retention)
	case http.MethodDelete:
		if err := wa.store.DeleteContainerRetention(containerName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) Run(addr string) error {
	if err := wa.loadContainers(); err != nil {
		return err
	}

	// Try to initialize database connection for EXPLAIN queries
	if err := sqlexplain.Init(); err != nil {
		slog.Warn("database connection not available (EXPLAIN feature disabled)", "error", err)
	} else {
		slog.Info("database connection established for EXPLAIN queries")
	}

	go wa.processLogs()
	go wa.monitorContainers()

	http.HandleFunc("/api/containers", wa.handleContainers)
	http.HandleFunc("/api/logs", wa.handleLogs)
	http.HandleFunc("/api/ws", wa.handleWebSocket)
	http.HandleFunc("/api/explain", wa.handleExplain)
	http.HandleFunc("/api/save-trace", wa.handleSaveTrace)
	http.HandleFunc("/debug", wa.handleDebug)

	// Request management endpoints
	http.HandleFunc("/api/servers", wa.handleServers)
	http.HandleFunc("/api/servers/", wa.handleServerDetail)
	http.HandleFunc("/api/database-urls", wa.handleDatabaseURLs)
	http.HandleFunc("/api/database-urls/", wa.handleDatabaseURLDetail)
	http.HandleFunc("/api/requests", wa.handleRequests)
	http.HandleFunc("/api/requests/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/execute") {
			wa.handleExecuteRequest(w, r)
		} else {
			wa.handleRequestDetail(w, r)
		}
	})
	http.HandleFunc("/api/executions", wa.handleExecutions)
	http.HandleFunc("/api/all-executions", wa.handleAllExecutions)
	http.HandleFunc("/api/executions/", wa.handleExecutionDetail)
	http.HandleFunc("/api/retention", wa.handleRetention)
	http.HandleFunc("/api/retention/", wa.handleRetentionDetail)

	http.Handle("/", http.FileServer(http.Dir("./web")))

	slog.Info("server starting", "address", "http://localhost"+addr)
	return http.ListenAndServe(addr, nil)
}

func main() {
	app, err := NewWebApp()
	if err != nil {
		slog.Error("failed to create app", "error", err)
		os.Exit(1)
	}

	defer sqlexplain.Close()
	if app.store != nil {
		defer app.store.Close()
	}

	if err := app.Run(":9000"); err != nil {
		slog.Error("failed to run server", "error", err)
		os.Exit(1)
	}
}
