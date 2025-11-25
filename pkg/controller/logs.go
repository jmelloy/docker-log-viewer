package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// LogWSMessage represents a log message sent over WebSocket
type LogWSMessage struct {
	ContainerID string         `json:"containerId"`
	Timestamp   time.Time      `json:"timestamp"`
	Entry       *logs.LogEntry `json:"entry"`
}

// HandleLogs returns recent logs
func (c *Controller) HandleLogs(w http.ResponseWriter, r *http.Request) {
	// Get recent logs from the store (limit to 1000)
	recentLogs := c.logStore.GetRecent(1000)

	logMessages := make([]LogWSMessage, 0, len(recentLogs))
	for _, logMsg := range recentLogs {
		logMessages = append(logMessages, LogWSMessage{
			ContainerID: logMsg.ContainerID,
			Timestamp:   logMsg.Timestamp,
			Entry:       deserializeLogEntry(logMsg),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logMessages)
}

// HandleClearLogs clears all logs from the log store
func (c *Controller) HandleClearLogs(w http.ResponseWriter, r *http.Request) {
	c.logStore.Clear()
	slog.Info("cleared all logs from log store")

	// Broadcast a clear message to all WebSocket clients
	c.clientsMutex.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for client := range c.clients {
		clients = append(clients, client)
	}
	c.clientsMutex.RUnlock()

	clearMsg := WSMessage{
		Type: "logs_clear",
		Data: json.RawMessage("[]"),
	}

	for _, client := range clients {
		if err := client.conn.WriteJSON(clearMsg); err != nil {
			slog.Error("failed to send clear message", "error", err)
			client.conn.Close()
			c.clientsMutex.Lock()
			delete(c.clients, client)
			c.clientsMutex.Unlock()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logs cleared successfully",
	})
}

// HandleWebSocket manages WebSocket connections for real-time log streaming
func (c *Controller) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
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

	c.clientsMutex.Lock()
	c.clients[client] = true
	c.clientsMutex.Unlock()

	defer func() {
		c.clientsMutex.Lock()
		delete(c.clients, client)
		c.clientsMutex.Unlock()
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
			go c.sendInitialLogs(client)
		}
	}
}

// sendInitialLogs sends filtered logs to a WebSocket client
func (c *Controller) sendInitialLogs(client *Client) {
	client.mu.RLock()
	filter := client.filter
	client.mu.RUnlock()

	filterOpts := c.clientFilterToLogStoreFilter(filter)
	recentStoreLogs := c.logStore.Filter(filterOpts, 10000)

	filteredLogs := make([]LogWSMessage, 0, len(recentStoreLogs))
	slices.Reverse(recentStoreLogs)

	for _, storeMsg := range recentStoreLogs {
		filteredLogs = append(filteredLogs, LogWSMessage{
			ContainerID: storeMsg.ContainerID,
			Timestamp:   storeMsg.Timestamp,
			Entry:       deserializeLogEntry(storeMsg),
		})
	}

	wsMsg := WSMessage{
		Type: "logs_initial",
	}
	data, err := json.Marshal(filteredLogs)
	if err != nil {
		slog.Error("failed to marshal filtered logs", "error", err)
		return
	}
	wsMsg.Data = data

	if err := client.conn.WriteJSON(wsMsg); err != nil {
		slog.Error("failed to send initial logs", "error", err)
	}
}

// clientFilterToLogStoreFilter converts a ClientFilter to logstore.FilterOptions
func (c *Controller) clientFilterToLogStoreFilter(filter ClientFilter) logstore.FilterOptions {
	opts := logstore.FilterOptions{}

	if len(filter.SelectedContainers) > 0 {
		c.containerMutex.RLock()
		containerIDs := make([]string, 0, len(filter.SelectedContainers))
		for containerID, containerName := range c.containerIDNames {
			if slices.Contains(filter.SelectedContainers, containerName) {
				containerIDs = append(containerIDs, containerID)
			}
		}
		c.containerMutex.RUnlock()
		opts.ContainerIDs = containerIDs
	}

	if len(filter.SelectedLevels) > 0 {
		opts.Levels = filter.SelectedLevels
	}

	if filter.SearchQuery != "" {
		opts.SearchTerms = strings.Fields(filter.SearchQuery)
	}

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

// matchesFilter checks if a log matches the client's filter criteria
func (c *Controller) matchesFilter(msg logs.LogMessage, filter ClientFilter) bool {
	if len(filter.SelectedContainers) > 0 {
		c.containerMutex.RLock()
		containerName := c.containerIDNames[msg.ContainerID]
		c.containerMutex.RUnlock()

		if !slices.Contains(filter.SelectedContainers, containerName) {
			return false
		}
	}

	if len(filter.SelectedLevels) > 0 {
		if msg.Entry == nil {
			return false
		}

		if msg.Entry.Level == "" {
			if !slices.Contains(filter.SelectedLevels, "NONE") {
				return false
			}
		} else {
			logLevel := strings.ToUpper(msg.Entry.Level)
			if !slices.Contains(filter.SelectedLevels, logLevel) {
				return false
			}
		}
	}

	if filter.SearchQuery != "" {
		terms := strings.Fields(filter.SearchQuery)

		if msg.Entry != nil {
			for _, term := range terms {
				query := strings.ToLower(term)
				found := false

				if strings.Contains(strings.ToLower(msg.Entry.Message), query) {
					found = true
				}

				if !found && strings.Contains(strings.ToLower(msg.Entry.Raw), query) {
					found = true
				}

				if !found && msg.Entry.Fields != nil {
					for key, value := range msg.Entry.Fields {
						if strings.Contains(strings.ToLower(key), query) || strings.Contains(strings.ToLower(value), query) {
							found = true
							break
						}
					}
				}

				if !found {
					return false
				}
			}
		} else {
			return false
		}
	}

	if len(filter.TraceFilters) > 0 && msg.Entry != nil && msg.Entry.Fields != nil {
		for _, tf := range filter.TraceFilters {
			if val, ok := msg.Entry.Fields[tf.Type]; !ok || val != tf.Value {
				return false
			}
		}
	}

	return true
}

// BroadcastBatch sends a batch of logs to all connected WebSocket clients
func (c *Controller) BroadcastBatch(batch []logs.LogMessage) {
	c.clientsMutex.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for client := range c.clients {
		clients = append(clients, client)
	}
	c.clientsMutex.RUnlock()

	if len(batch) == 0 {
		return
	}

	for _, client := range clients {
		client.mu.RLock()
		filter := client.filter
		client.mu.RUnlock()

		filteredLogs := []LogWSMessage{}
		for _, msg := range batch {
			if c.matchesFilter(msg, filter) {
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
		data, err := json.Marshal(filteredLogs)
		if err != nil {
			slog.Error("failed to marshal logs batch", "error", err)
			continue
		}
		wsMsg.Data = data

		if err := client.conn.WriteJSON(wsMsg); err != nil {
			// Close connection on error and remove from clients map
			client.conn.Close()

			c.clientsMutex.Lock()
			delete(c.clients, client)
			c.clientsMutex.Unlock()
		}
	}
}

// Helper functions for log entry serialization

// serializeLogEntry converts a logs.LogEntry into fields for logstore
func serializeLogEntry(entry *logs.LogEntry) (message string, fields map[string]string) {
	if entry == nil {
		return "", make(map[string]string)
	}

	fields = make(map[string]string)
	message = entry.Message

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

	for k, v := range entry.Fields {
		fields[k] = v
	}

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

	for k, v := range msg.Fields {
		if !strings.HasPrefix(k, "_") {
			entry.Fields[k] = v
		}
	}

	return entry
}
