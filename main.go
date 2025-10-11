package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"docker-log-parser/pkg/logs"
	"github.com/gorilla/websocket"
)

type WebApp struct {
	docker       *logs.DockerClient
	logs         []logs.LogMessage
	logsMutex    sync.RWMutex
	containers   []logs.Container
	clients      map[*websocket.Conn]bool
	clientsMutex sync.RWMutex
	logChan      chan logs.LogMessage
	ctx          context.Context
	cancel       context.CancelFunc
	upgrader     websocket.Upgrader
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ContainersUpdateMessage struct {
	Containers []logs.Container `json:"containers"`
}

type LogWSMessage struct {
	ContainerID string         `json:"containerId"`
	Timestamp   time.Time      `json:"timestamp"`
	Entry       *logs.LogEntry `json:"entry"`
}

func NewWebApp() (*WebApp, error) {
	docker, err := logs.NewDockerClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &WebApp{
		docker:  docker,
		logs:    make([]logs.LogMessage, 0),
		clients: make(map[*websocket.Conn]bool),
		logChan: make(chan logs.LogMessage, 1000),
		ctx:     ctx,
		cancel:  cancel,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	return app, nil
}

func (wa *WebApp) handleContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func (wa *WebApp) handleLogs(w http.ResponseWriter, r *http.Request) {
	wa.logsMutex.RLock()
	defer wa.logsMutex.RUnlock()

	startIdx := 0
	if len(wa.logs) > 1000 {
		startIdx = len(wa.logs) - 1000
	}

	logs := make([]LogWSMessage, 0)
	for i := startIdx; i < len(wa.logs); i++ {
		msg := wa.logs[i]
		logs = append(logs, LogWSMessage{
			ContainerID: msg.ContainerID,
			Timestamp:   msg.Timestamp,
			Entry:       msg.Entry,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (wa *WebApp) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wa.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	wa.clientsMutex.Lock()
	wa.clients[conn] = true
	wa.clientsMutex.Unlock()

	defer func() {
		wa.clientsMutex.Lock()
		delete(wa.clients, conn)
		wa.clientsMutex.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (wa *WebApp) broadcastLog(msg logs.LogMessage) {
	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	logMsg := LogWSMessage{
		ContainerID: msg.ContainerID,
		Timestamp:   msg.Timestamp,
		Entry:       msg.Entry,
	}

	wsMsg := WSMessage{
		Type: "log",
	}
	data, _ := json.Marshal(logMsg)
	wsMsg.Data = data

	for client := range wa.clients {
		err := client.WriteJSON(wsMsg)
		if err != nil {
			client.Close()
			delete(wa.clients, client)
		}
	}
}

func (wa *WebApp) processLogs() {
	for {
		select {
		case <-wa.ctx.Done():
			return
		case msg := <-wa.logChan:
			wa.logsMutex.Lock()
			wa.logs = append(wa.logs, msg)
			if len(wa.logs) > 10000 {
				wa.logs = wa.logs[1000:]
			}
			wa.logsMutex.Unlock()

			wa.broadcastLog(msg)
		}
	}
}

func (wa *WebApp) loadContainers() error {
	containers, err := wa.docker.ListRunningContainers(wa.ctx)
	if err != nil {
		return err
	}

	wa.containers = containers

	for _, c := range containers {
		if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
			log.Printf("Failed to stream logs for container %s: %v", c.ID, err)
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
				log.Printf("Failed to list containers: %v", err)
				continue
			}

			currentIDs := make(map[string]bool)
			for _, c := range containers {
				currentIDs[c.ID] = true
			}

			for _, c := range containers {
				if !previousIDs[c.ID] {
					log.Printf("New container detected: %s (%s)", c.ID, c.Name)
					if err := wa.docker.StreamLogs(wa.ctx, c.ID, wa.logChan); err != nil {
						log.Printf("Failed to stream logs for new container %s: %v", c.ID, err)
					}
				}
			}

			for id := range previousIDs {
				if !currentIDs[id] {
					log.Printf("Container stopped: %s", id)
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

func (wa *WebApp) broadcastContainerUpdate(containers []logs.Container) {
	update := ContainersUpdateMessage{
		Containers: containers,
	}

	wsMsg := WSMessage{
		Type: "containers",
	}
	data, _ := json.Marshal(update)
	wsMsg.Data = data

	wa.clientsMutex.RLock()
	defer wa.clientsMutex.RUnlock()

	for client := range wa.clients {
		err := client.WriteJSON(wsMsg)
		if err != nil {
			client.Close()
			delete(wa.clients, client)
		}
	}
}

func (wa *WebApp) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExplainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := ExplainQuery(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (wa *WebApp) Run(addr string) error {
	if err := wa.loadContainers(); err != nil {
		return err
	}

	// Try to initialize database connection for EXPLAIN queries
	if err := InitDB(); err != nil {
		log.Printf("Database connection not available (EXPLAIN feature disabled): %v", err)
	} else {
		log.Printf("Database connection established for EXPLAIN queries")
	}

	go wa.processLogs()
	go wa.monitorContainers()

	http.HandleFunc("/api/containers", wa.handleContainers)
	http.HandleFunc("/api/logs", wa.handleLogs)
	http.HandleFunc("/api/ws", wa.handleWebSocket)
	http.HandleFunc("/api/explain", wa.handleExplain)
	http.Handle("/", http.FileServer(http.Dir("./web")))

	log.Printf("Server starting on http://localhost:%s", addr)
	return http.ListenAndServe(addr, nil)
}

func main() {
	app, err := NewWebApp()
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	defer CloseDB()

	if err := app.Run(":9000"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
