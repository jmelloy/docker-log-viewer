package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"

	"github.com/gorilla/websocket"
)

// HandleContainers returns a list of running containers with their log counts and retention settings
func (wa *WebApp) HandleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := wa.Docker.ListRunningContainers(wa.Ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	portToServerMap := wa.BuildPortToServerMap(containers)

	// Get log counts for each container
	logCounts := make(map[string]int)
	for _, container := range containers {
		count := wa.LogStore.CountByContainer(container.ID)
		logCounts[container.Name] = count
	}

	// Get retention settings for all containers
	retentions := make(map[string]RetentionInfo)
	if wa.Store != nil {
		retentionList, err := wa.Store.ListContainerRetentions()
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

// HandleLogs returns recent logs from the store
func (wa *WebApp) HandleLogs(w http.ResponseWriter, r *http.Request) {
	// Get recent logs from the store (limit to 1000)
	recentLogs := wa.LogStore.GetRecent(1000)

	logs := make([]LogWSMessage, 0, len(recentLogs))
	for _, logMsg := range recentLogs {
		logs = append(logs, LogWSMessage{
			ContainerID: logMsg.ContainerID,
			Timestamp:   logMsg.Timestamp,
			Entry:       DeserializeLogEntry(logMsg),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// HandleDebug returns debug information about the application state
func (wa *WebApp) HandleDebug(w http.ResponseWriter, r *http.Request) {
	totalLogs := wa.LogStore.Count()

	// Count logs by container
	logsByContainer := make(map[string]int)

	wa.ContainerMutex.RLock()
	containers := make([]map[string]string, 0)
	for id, name := range wa.ContainerIDNames {
		shortID := id
		if len(id) > 12 {
			shortID = id[:12]
		}
		// Get count from logstore for this container
		count := wa.LogStore.CountByContainer(id)
		logsByContainer[id] = count

		containers = append(containers, map[string]string{
			"id":    shortID,
			"name":  name,
			"count": fmt.Sprintf("%d", count),
		})
	}
	wa.ContainerMutex.RUnlock()

	wa.ClientsMutex.RLock()
	clientCount := len(wa.Clients)
	clientFilters := make([]map[string]interface{}, 0)
	for client := range wa.Clients {
		clientFilters = append(clientFilters, map[string]interface{}{
			"selectedContainers": client.Filter.SelectedContainers,
			"selectedLevels":     client.Filter.SelectedLevels,
			"searchQuery":        client.Filter.SearchQuery,
			"traceFilterCount":   len(client.Filter.TraceFilters),
		})
	}
	wa.ClientsMutex.RUnlock()

	debugInfo := map[string]interface{}{
		"totalLogsInMemory": totalLogs,
		"containerCount":    len(containers),
		"containers":        containers,
		"connectedClients":  clientCount,
		"clientFilters":     clientFilters,
		"logChannelSize":    len(wa.LogChan),
		"logChannelCap":     cap(wa.LogChan),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// HandleWebSocket handles WebSocket connections for real-time log streaming
func (wa *WebApp) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wa.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error", "error", err)
		return
	}

	client := &Client{
		Conn: conn,
		Filter: ClientFilter{
			SelectedContainers: []string{},
			SelectedLevels:     []string{},
			SearchQuery:        "",
			TraceFilters:       []TraceFilterValue{},
		},
	}

	wa.ClientsMutex.Lock()
	wa.Clients[client] = true
	wa.ClientsMutex.Unlock()

	defer func() {
		wa.ClientsMutex.Lock()
		delete(wa.Clients, client)
		wa.ClientsMutex.Unlock()
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
			client.Mu.Lock()
			client.Filter = filter
			client.Mu.Unlock()

			// Send initial filtered logs to the client
			go wa.SendInitialLogs(client)
		}
	}
}

// SendInitialLogs sends filtered logs to a newly connected or updated client
func (wa *WebApp) SendInitialLogs(client *Client) {
	client.Mu.RLock()
	filter := client.Filter
	client.Mu.RUnlock()

	// Convert client filter to LogStore FilterOptions
	filterOpts := wa.ClientFilterToLogStoreFilter(filter)

	// Use LogStore's Filter method to get filtered logs directly
	recentStoreLogs := wa.LogStore.Filter(filterOpts, 10000)

	// Convert to WebSocket format
	filteredLogs := make([]LogWSMessage, 0, len(recentStoreLogs))
	slices.Reverse(recentStoreLogs)

	for _, storeMsg := range recentStoreLogs {
		filteredLogs = append(filteredLogs, LogWSMessage{
			ContainerID: storeMsg.ContainerID,
			Timestamp:   storeMsg.Timestamp,
			Entry:       DeserializeLogEntry(storeMsg),
		})
	}

	slog.Info("filteredLogs", "count", len(filteredLogs))
	// Send clear message to replace all logs
	wsMsg := WSMessage{
		Type: "logs_initial",
	}
	data, _ := json.Marshal(filteredLogs)
	wsMsg.Data = data

	client.Conn.WriteJSON(wsMsg)
}

// ClientFilterToLogStoreFilter converts a ClientFilter to logstore.FilterOptions
func (wa *WebApp) ClientFilterToLogStoreFilter(filter ClientFilter) logstore.FilterOptions {
	opts := logstore.FilterOptions{}

	// Convert container names to container IDs
	if len(filter.SelectedContainers) > 0 {
		wa.ContainerMutex.RLock()
		containerIDs := make([]string, 0, len(filter.SelectedContainers))
		for containerID, containerName := range wa.ContainerIDNames {
			if slices.Contains(filter.SelectedContainers, containerName) {
				containerIDs = append(containerIDs, containerID)
			}
		}
		wa.ContainerMutex.RUnlock()
		opts.ContainerIDs = containerIDs
	}

	// Set levels
	if len(filter.SelectedLevels) > 0 {
		opts.Levels = filter.SelectedLevels
	}

	// Set search terms (split by whitespace for AND logic)
	if filter.SearchQuery != "" {
		opts.SearchTerms = strings.Fields(filter.SearchQuery)
	}

	// Set trace filters as field filters
	if len(filter.TraceFilters) > 0 {
		fieldFilters := make([]logstore.FieldFilter, 0, len(filter.TraceFilters))
		for _, tf := range filter.TraceFilters {
			fieldFilters = append(fieldFilters, logstore.FieldFilter{
				Name:  tf.Type,
				Value: tf.Value,
			})
		}
		opts.FieldFilters = fieldFilters
	}

	return opts
}

// MatchesFilter checks if a log matches the client's filter criteria (including container filter)
func (wa *WebApp) MatchesFilter(msg logs.LogMessage, filter ClientFilter) bool {
	// Container filter
	if len(filter.SelectedContainers) > 0 {
		wa.ContainerMutex.RLock()
		containerName := wa.ContainerIDNames[msg.ContainerID]
		wa.ContainerMutex.RUnlock()

		if !slices.Contains(filter.SelectedContainers, containerName) {
			return false
		}
	}

	// Level filter
	if len(filter.SelectedLevels) > 0 {
		if msg.Entry == nil {
			return false
		}

		// Check if log has a level
		if msg.Entry.Level == "" {
			// No level parsed - check if NONE is selected
			if !slices.Contains(filter.SelectedLevels, "NONE") {
				return false
			}
		} else {
			// Has a level - check if it matches (case-insensitive)
			logLevel := strings.ToUpper(msg.Entry.Level)
			if !slices.Contains(filter.SelectedLevels, logLevel) {
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

// BuildPortToServerMap builds a mapping of container ports to database connection strings
func (wa *WebApp) BuildPortToServerMap(containers []logs.Container) map[int]string {
	portToServerMap := make(map[int]string)

	if wa.Store == nil {
		return portToServerMap
	}

	// Get all servers with default databases
	servers, err := wa.Store.ListServers()
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
