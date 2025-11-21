package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

// RetentionInfo represents container retention settings
type RetentionInfo struct {
	Type  string `json:"type"`  // "count" or "time"
	Value int    `json:"value"` // number of logs or seconds
}

// ContainersUpdateMessage represents the containers update response
type ContainersUpdateMessage struct {
	Containers      []logs.Container         `json:"containers"`
	PortToServerMap map[int]string           `json:"portToServerMap"`
	LogCounts       map[string]int           `json:"logCounts"`
	Retentions      map[string]RetentionInfo `json:"retentions"`
}

// HandleContainers lists all running containers with associated metadata
func (c *Controller) HandleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := c.docker.ListRunningContainers(c.ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	portToServerMap := c.buildPortToServerMap(containers)

	// Get log counts for each container
	logCounts := make(map[string]int)
	for _, container := range containers {
		count := c.logStore.CountByContainer(container.ID)
		logCounts[container.Name] = count
	}

	// Get retention settings for all containers
	retentions := make(map[string]RetentionInfo)
	if c.store != nil {
		retentionList, err := c.store.ListContainerRetentions()
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

// HandleDebug returns debug information about the system state
func (c *Controller) HandleDebug(w http.ResponseWriter, r *http.Request) {
	totalLogs := c.logStore.Count()

	// Count logs by container
	logsByContainer := make(map[string]int)

	c.containerMutex.RLock()
	containers := make([]map[string]string, 0)
	for id, name := range c.containerIDNames {
		shortID := id
		if len(id) > 12 {
			shortID = id[:12]
		}
		count := c.logStore.CountByContainer(id)
		logsByContainer[id] = count

		containers = append(containers, map[string]string{
			"id":    shortID,
			"name":  name,
			"count": fmt.Sprintf("%d", count),
		})
	}
	c.containerMutex.RUnlock()

	c.clientsMutex.RLock()
	clientCount := len(c.clients)
	clientFilters := make([]map[string]interface{}, 0)
	for client := range c.clients {
		clientFilters = append(clientFilters, map[string]interface{}{
			"selectedContainers": client.filter.SelectedContainers,
			"selectedLevels":     client.filter.SelectedLevels,
			"searchQuery":        client.filter.SearchQuery,
			"traceFilterCount":   len(client.filter.TraceFilters),
		})
	}
	c.clientsMutex.RUnlock()

	debugInfo := map[string]interface{}{
		"totalLogsInMemory": totalLogs,
		"containerCount":    len(containers),
		"containers":        containers,
		"connectedClients":  clientCount,
		"clientFilters":     clientFilters,
		"logChannelSize":    len(c.logChan),
		"logChannelCap":     cap(c.logChan),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

// HandleRetention manages container retention settings
func (c *Controller) HandleRetention(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retentions, err := c.store.ListContainerRetentions()
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
		if err := c.store.SaveContainerRetention(&retention); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		c.containerMutex.RLock()
		containers := c.containers
		c.containerMutex.RUnlock()

		for _, container := range containers {
			if container.Name == retention.ContainerName {
				containerID := container.ID
				slog.Info("setting container retention", "containerID", containerID, "retention", retention)
				c.logStore.SetContainerRetention(containerID, logstore.ContainerRetentionPolicy{
					Type:  retention.RetentionType,
					Value: retention.RetentionValue,
				})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retention)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleRetentionDetail manages a specific container's retention settings
func (c *Controller) HandleRetentionDetail(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	containerName := vars["containerName"]
	if containerName == "" {
		http.Error(w, "Container name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retention, err := c.store.GetContainerRetention(containerName)
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
		if err := c.store.DeleteContainerRetention(containerName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// buildPortToServerMap builds a map of container ports to database connection strings
func (c *Controller) buildPortToServerMap(containers []logs.Container) map[int]string {
	portToServerMap := make(map[int]string)

	if c.store == nil {
		return portToServerMap
	}

	servers, err := c.store.ListServers()
	if err != nil {
		slog.Error("failed to list servers for port mapping", "error", err)
		return portToServerMap
	}

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

// broadcastContainerUpdate sends container updates to all connected WebSocket clients
func (c *Controller) broadcastContainerUpdate(containers []logs.Container) {
	portToServerMap := c.buildPortToServerMap(containers)

	logCounts := make(map[string]int)
	for _, container := range containers {
		count := c.logStore.CountByContainer(container.ID)
		logCounts[container.Name] = count
	}

	retentions := make(map[string]RetentionInfo)
	if c.store != nil {
		retentionList, err := c.store.ListContainerRetentions()
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

	c.clientsMutex.Lock()
	defer c.clientsMutex.Unlock()

	for client := range c.clients {
		err := client.conn.WriteJSON(wsMsg)
		if err != nil {
			client.conn.Close()
			delete(c.clients, client)
		}
	}
}

// LoadContainerRetentions loads retention settings from database
func (c *Controller) LoadContainerRetentions() error {
	if c.store == nil {
		return nil
	}

	retentionList, err := c.store.ListContainerRetentions()
	if err != nil {
		return err
	}

	c.containerMutex.RLock()
	containers := c.containers
	c.containerMutex.RUnlock()

	for _, retention := range retentionList {
		for _, container := range containers {
			if container.Name == retention.ContainerName {
				containerID := container.ID
				slog.Info("setting container retention", "containerID", containerID, "retention", retention)
				c.logStore.SetContainerRetention(containerID, logstore.ContainerRetentionPolicy{
					Type:  retention.RetentionType,
					Value: retention.RetentionValue,
				})
			}
		}
	}
	return nil
}
