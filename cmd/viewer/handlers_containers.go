package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
)

// ============================================================================
// HTTP Handlers - Container & Log Management
// ============================================================================

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

	// Convert client filter to LogStore FilterOptions
	filterOpts := wa.clientFilterToLogStoreFilter(filter)

	// Use LogStore's Filter method to get filtered logs directly
	recentStoreLogs := wa.logStore.Filter(filterOpts, 10000)

	// Convert to WebSocket format
	filteredLogs := make([]LogWSMessage, 0, len(recentStoreLogs))
	slices.Reverse(recentStoreLogs)

	for _, storeMsg := range recentStoreLogs {
		filteredLogs = append(filteredLogs, LogWSMessage{
			ContainerID: storeMsg.ContainerID,
			Timestamp:   storeMsg.Timestamp,
			Entry:       deserializeLogEntry(storeMsg),
		})
	}

	slog.Info("filteredLogs", "count", len(filteredLogs))
	// Send clear message to replace all logs
	wsMsg := WSMessage{
		Type: "logs_initial",
	}
	data, _ := json.Marshal(filteredLogs)
	wsMsg.Data = data

	client.conn.WriteJSON(wsMsg)
}

// clientFilterToLogStoreFilter converts a ClientFilter to logstore.FilterOptions
func (wa *WebApp) clientFilterToLogStoreFilter(filter ClientFilter) logstore.FilterOptions {
	opts := logstore.FilterOptions{}

	// Convert container names to container IDs
	if len(filter.SelectedContainers) > 0 {
		wa.containerMutex.RLock()
		containerIDs := make([]string, 0, len(filter.SelectedContainers))
		for containerID, containerName := range wa.containerIDNames {
			if slices.Contains(filter.SelectedContainers, containerName) {
				containerIDs = append(containerIDs, containerID)
			}
		}
		wa.containerMutex.RUnlock()
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

// matchesFilter checks if a log matches the client's filter criteria (including container filter)
func (wa *WebApp) matchesFilter(msg logs.LogMessage, filter ClientFilter) bool {
	// Container filter
	if len(filter.SelectedContainers) > 0 {
		wa.containerMutex.RLock()
		containerName := wa.containerIDNames[msg.ContainerID]
		wa.containerMutex.RUnlock()

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
